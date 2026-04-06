package internals

import (
	"context"
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
)

func parseFile(path string, config *LanguageConfig) (FileInfo, error) {
	config.ensureCompiled()

	src, err := os.ReadFile(path)
	if err != nil {
		return FileInfo{}, err
	}

	if config.SrcTransform != nil {
		src = config.SrcTransform(src)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(config.Grammar)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return FileInfo{}, err
	}
	root := tree.RootNode()
	if root.HasError() {
		fmt.Fprintf(os.Stderr, "syntax errors in %s\n", path)
	}

	fi := FileInfo{Language: config.Name}
	fi.Namespace = extractNamespace(root, src, config)
	fi.Symbols = extractSymbols(root, src, config)
	if config.PostProcess != nil {
		fi.Symbols = config.PostProcess(fi.Symbols)
	}
	return fi, nil
}

func extractNamespace(root *sitter.Node, src []byte, config *LanguageConfig) string {
	if config.compiledNamespaceQuery == nil {
		return ""
	}

	cur := sitter.NewQueryCursor()
	cur.Exec(config.compiledNamespaceQuery, root)
	if m, ok := cur.NextMatch(); ok {
		for _, c := range m.Captures {
			if config.compiledNamespaceQuery.CaptureNameForId(c.Index) == "name" {
				return c.Node.Content(src)
			}
		}
	}
	return ""
}

func extractSymbols(root *sitter.Node, src []byte, config *LanguageConfig) []Symbol {
	var raws []rawSym

	for _, sq := range config.compiledSymbolQueries {
		cur := sitter.NewQueryCursor()
		cur.Exec(sq.Query, root)

		for {
			m, ok := cur.NextMatch()
			if !ok {
				break
			}

			var nameNode, declNode *sitter.Node
			for _, c := range m.Captures {
				switch sq.Query.CaptureNameForId(c.Index) {
				case "name":
					nameNode = c.Node
				case "decl":
					declNode = c.Node
				}
			}
			if nameNode == nil {
				continue
			}

			rangeNode := declarationRangeNode(nameNode, declNode)
			var annotations []Annotation
			if declNode != nil {
				annotations = extractAnnotationsFromDecl(declNode, src, config)
			}

			raws = append(raws, rawSym{
				name:        nameNode.Content(src),
				kind:        sq.Kind,
				line:        nameNode.StartPoint().Row + 1,
				startByte:   rangeNode.StartByte(),
				endByte:     rangeNode.EndByte(),
				isContainer: sq.IsContainer,
				annotations: annotations,
			})
		}
	}

	return buildHierarchy(raws)
}

func declarationRangeNode(nameNode, declNode *sitter.Node) *sitter.Node {
	if declNode != nil {
		return declNode
	}
	if parent := nameNode.Parent(); parent != nil {
		return parent
	}
	return nameNode
}
