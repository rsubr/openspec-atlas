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

	props := extractReactProps(src)
	lineIndex := buildLineIndex(src)

	var components []UIComponent
	seen := map[string]bool{}
	for _, m := range matches {
		name := reactComponentName(src, m)
		if name == "" || seen[name] || isAllCapsIdentifier(name) {
			continue
		}
		seen[name] = true
		components = append(components, UIComponent{
			Name:      name,
			Framework: UIFrameworkReact,
			File:      file,
			Line:      lineForOffset(lineIndex, m[0]),
			Props:     props,
		})
	}
	return components
}

func extractReactProps(src []byte) []string {
	var props []string
	if pm := reactPropsInterfaceRe.FindSubmatch(src); pm != nil {
		scanner := bufio.NewScanner(bytes.NewReader(pm[1]))
		for scanner.Scan() {
			if fm := reactPropLineRe.FindStringSubmatch(scanner.Text()); fm != nil {
				props = append(props, fm[1])
			}
		}
	}
	return props
}

func reactComponentName(src []byte, match []int) string {
	if match[2] >= 0 {
		return string(src[match[2]:match[3]])
	}
	if match[4] >= 0 {
		return string(src[match[4]:match[5]])
	}
	return ""
}

func isAllCapsIdentifier(name string) bool {
	return name == strings.ToUpper(name)
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
		Framework: UIFrameworkSvelte,
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
				Framework: UIFrameworkAngular,
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
		name := vueComponentName(fi.Path)
		if name == "" {
			continue
		}
		comp := UIComponent{
			Name:      name,
			Framework: UIFrameworkVue,
			File:      fi.Path,
			Line:      1,
		}
		comp.Props = collectVueProps(fi.Symbols)
		components = append(components, comp)
	}
	return components
}

func vueComponentName(path string) string {
	base := path
	if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
		base = base[idx+1:]
	}
	if !strings.HasSuffix(base, ".vue") {
		return ""
	}
	raw := base[:len(base)-4]
	if raw == "" {
		return ""
	}
	return strings.ToUpper(raw[:1]) + raw[1:]
}

func collectVueProps(symbols []Symbol) []string {
	var props []string
	for _, sym := range symbols {
		if sym.Name != "props" && sym.Name != "defineProps" {
			continue
		}
		for _, child := range sym.Children {
			props = append(props, child.Name)
		}
	}
	return props
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
