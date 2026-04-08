package internal

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// extractAnnotationsFromDecl finds annotation nodes scoped to the given
// declaration and runs the pre-compiled annotation queries against them.
//
// Scoping strategy (avoids picking up nested annotations from child declarations):
//
//	AnnotationContainerType = "modifiers" / "attribute_list"
//	    → find the named container as a direct child of declNode and query it.
//	      Multiple container siblings (e.g. C# [Attr1][Attr2]) are all queried.
//
//	AnnotationContainerType = "" + AnnotationNodeTypes set
//	    → collect annotation nodes (e.g. "decorator") that are direct children
//	      of declNode and query each one.
//
//	AnnotationContainerType = "parent" + AnnotationNodeTypes set
//	    → same as above but from the parent node (Python: decorated_definition
//	      wraps function_definition; decorators live on the parent).
func extractAnnotationsFromDecl(declNode *sitter.Node, src []byte, config *LanguageConfig) []Annotation {
	if len(config.compiledAnnQueries) == 0 {
		return nil
	}

	targets := annotationTargets(declNode, config)
	if len(targets) == 0 {
		return nil
	}

	seen := map[string]bool{}
	var annotations []Annotation

	for _, target := range targets {
		for _, q := range config.compiledAnnQueries {
			cur := sitter.NewQueryCursor()
			cur.Exec(q, target)
			for {
				m, ok := cur.NextMatch()
				if !ok {
					break
				}
				var name, value string
				for _, c := range m.Captures {
					switch q.CaptureNameForId(c.Index) {
					case "name":
						name = c.Node.Content(src)
					case "value":
						value = strings.Trim(c.Node.Content(src), `"'`)
					}
				}
				if name == "" {
					continue
				}
				key := name + ":" + value
				if seen[key] {
					continue
				}
				seen[key] = true
				annotations = append(annotations, Annotation{Name: name, Value: value})
			}
		}
	}

	return annotations
}

func annotationTargets(declNode *sitter.Node, config *LanguageConfig) []*sitter.Node {
	switch config.AnnotationContainerType {
	case "parent":
		if parent := declNode.Parent(); parent != nil {
			return directChildrenOfType(parent, config.AnnotationNodeTypes)
		}
	case "":
		if len(config.AnnotationNodeTypes) > 0 {
			return directChildrenOfType(declNode, config.AnnotationNodeTypes)
		}
	default:
		return directChildrenOfType(declNode, []string{config.AnnotationContainerType})
	}
	return nil
}

// directChildrenOfType returns direct children of node whose type is in the set.
func directChildrenOfType(node *sitter.Node, types []string) []*sitter.Node {
	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	var result []*sitter.Node
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if typeSet[child.Type()] {
			result = append(result, child)
		}
	}
	return result
}
