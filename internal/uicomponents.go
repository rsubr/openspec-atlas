package internal

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// ---- React -----------------------------------------------------------------

// reactComponentRe matches exported PascalCase functions / arrow functions
// that likely return JSX.  We look for patterns like:
//
//	export function MyComponent(
//	export const MyComponent = (
//	export default function MyComponent(
var reactComponentRe = regexp.MustCompile(
	`(?m)^(?:export\s+(?:default\s+)?)?` +
		`(?:function\s+([A-Z]\w*)\s*\(|const\s+([A-Z]\w*)\s*=\s*(?:React\.memo\(|React\.forwardRef\(|\())`)

// jsxReturnRe detects that the file actually uses JSX (presence of JSX tags).
var jsxReturnRe = regexp.MustCompile(`return\s*\(?\s*<[A-Z][A-Za-z]|return\s*<[a-z]`)

// reactPropsRe extracts a props interface or type.
// Matches: interface MyComponentProps { ... } or type MyComponentProps = { ... }
var reactPropsInterfaceRe = regexp.MustCompile(`(?m)^(?:interface|type)\s+\w+Props(?:\s*=)?\s*\{([^}]+)\}`)
var reactPropLineRe = regexp.MustCompile(`^\s*(\w+)\??:`)

func extractReactComponents(src []byte, file string) []UIComponent {
	if !jsxReturnRe.Match(src) && !bytes.Contains(src, []byte("React")) {
		return nil
	}
	matches := reactComponentRe.FindAllSubmatchIndex(src, -1)
	if len(matches) == 0 {
		return nil
	}

	// Extract prop names from Props interface/type in the file
	var props []string
	if pm := reactPropsInterfaceRe.FindSubmatch(src); pm != nil {
		scanner := bufio.NewScanner(bytes.NewReader(pm[1]))
		for scanner.Scan() {
			if fm := reactPropLineRe.FindStringSubmatch(scanner.Text()); fm != nil {
				props = append(props, fm[1])
			}
		}
	}

	lines := splitLines(src)
	lineIndex := buildLineIndex(src)

	var components []UIComponent
	seen := map[string]bool{}
	for _, m := range matches {
		name := ""
		if m[2] >= 0 { // function MyComponent
			name = string(src[m[2]:m[3]])
		} else if m[4] >= 0 { // const MyComponent
			name = string(src[m[4]:m[5]])
		}
		if name == "" || seen[name] {
			continue
		}
		// Skip all-caps identifiers like URL, HTML, API (likely constants)
		if name == strings.ToUpper(name) {
			continue
		}
		seen[name] = true
		lineNum := lineForOffset(lineIndex, m[0])
		_ = lines
		components = append(components, UIComponent{
			Name:      name,
			Framework: "react",
			File:      file,
			Line:      lineNum,
			Props:     props,
		})
	}
	return components
}

// ---- Svelte ----------------------------------------------------------------

// sveltePropsRe matches `export let propName` in a Svelte <script> block.
var sveltePropsRe = regexp.MustCompile(`(?m)^\s*export\s+let\s+(\w+)`)
var svelteScriptRe = regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`)

func extractSvelteComponent(src []byte, file string) *UIComponent {
	name := svelteComponentName(file)
	if name == "" {
		return nil
	}
	comp := &UIComponent{
		Name:      name,
		Framework: "svelte",
		File:      file,
		Line:      1,
	}

	// Extract props from <script> block
	if sm := svelteScriptRe.FindSubmatch(src); sm != nil {
		for _, pm := range sveltePropsRe.FindAllSubmatch(sm[1], -1) {
			comp.Props = append(comp.Props, string(pm[1]))
		}
	}
	return comp
}

// svelteComponentName derives a PascalCase component name from a .svelte filename.
func svelteComponentName(path string) string {
	base := path
	if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
		base = base[idx+1:]
	}
	if !strings.HasSuffix(base, ".svelte") {
		return ""
	}
	name := base[:len(base)-7]
	if len(name) == 0 {
		return ""
	}
	// Ensure PascalCase: capitalize first letter
	return strings.ToUpper(name[:1]) + name[1:]
}

// ---- Angular ---------------------------------------------------------------

// Angular components are detected from already-parsed symbol annotations.
// A class with @Component() is an Angular component; @Input() properties
// are treated as props.
func extractAngularComponents(files []FileInfo) []UIComponent {
	var components []UIComponent
	for _, fi := range files {
		for _, sym := range fi.Symbols {
			if sym.Kind != "class" {
				continue
			}
			if !hasAnnotation(sym.Annotations, "Component") {
				continue
			}
			comp := UIComponent{
				Name:      sym.Name,
				Framework: "angular",
				File:      fi.Path,
				Line:      int(sym.Line),
			}
			for _, child := range sym.Children {
				if hasAnnotation(child.Annotations, "Input") {
					comp.Props = append(comp.Props, child.Name)
				}
			}
			components = append(components, comp)
		}
	}
	return components
}

// ---- Vue -------------------------------------------------------------------

// Vue components are already handled by the existing Vue language config which
// parses the <script> block. We promote those parsed symbols into UIComponent
// entries so they appear in the unified ui_components inventory.
func extractVueComponents(files []FileInfo) []UIComponent {
	var components []UIComponent
	for _, fi := range files {
		if fi.Language != "vue" {
			continue
		}
		// Each Vue SFC is one component; use the filename as the component name.
		name := svelteComponentName(strings.TrimSuffix(fi.Path, ".ts") + ".svelte")
		// Re-derive name from .vue path
		base := fi.Path
		if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
			base = base[idx+1:]
		}
		if strings.HasSuffix(base, ".vue") {
			raw := base[:len(base)-4]
			name = strings.ToUpper(raw[:1]) + raw[1:]
		}
		if name == "" {
			continue
		}
		comp := UIComponent{
			Name:      name,
			Framework: "vue",
			File:      fi.Path,
			Line:      1,
		}
		// Collect exported props (defineProps or data) from symbols
		for _, sym := range fi.Symbols {
			if sym.Name == "props" || sym.Name == "defineProps" {
				for _, child := range sym.Children {
					comp.Props = append(comp.Props, child.Name)
				}
			}
		}
		components = append(components, comp)
	}
	return components
}

// ---- Dispatcher ------------------------------------------------------------

func collectUIComponents(allPaths []string, files []FileInfo, displayRoot string) []UIComponent {
	var components []UIComponent

	// Angular and Vue: sourced from already-parsed symbols
	components = append(components, extractAngularComponents(files)...)
	components = append(components, extractVueComponents(files)...)

	// React and Svelte: regex scan over raw source
	for _, path := range allPaths {
		ext := strings.ToLower(fileExt(path))

		src, err := readFileSafe(path)
		if err != nil || src == nil {
			continue
		}
		displayPath := relativePath(path, displayRoot)

		switch ext {
		case ".tsx", ".jsx":
			components = append(components, extractReactComponents(src, displayPath)...)
		case ".svelte":
			if comp := extractSvelteComponent(src, displayPath); comp != nil {
				components = append(components, *comp)
			}
		}
	}
	return components
}

