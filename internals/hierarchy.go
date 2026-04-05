package internals

import "sort"

func buildHierarchy(raws []rawSym) []Symbol {
	var containers, leaves []rawSym
	for _, r := range raws {
		if r.isContainer {
			containers = append(containers, r)
		} else {
			leaves = append(leaves, r)
		}
	}

	sort.Slice(containers, func(i, j int) bool {
		return containers[i].startByte < containers[j].startByte
	})

	containerSyms := make([]Symbol, len(containers))
	for i, c := range containers {
		containerSyms[i] = Symbol{
			Name:        c.name,
			Kind:        c.kind,
			Line:        c.line,
			Annotations: c.annotations,
		}
	}

	var topLevel []Symbol
	for _, leaf := range leaves {
		bestIdx := smallestContainingContainer(containers, leaf)
		sym := Symbol{
			Name:        leaf.name,
			Kind:        leaf.kind,
			Line:        leaf.line,
			Annotations: leaf.annotations,
		}
		if bestIdx >= 0 {
			containerSyms[bestIdx].Children = append(containerSyms[bestIdx].Children, sym)
		} else {
			topLevel = append(topLevel, sym)
		}
	}

	return append(containerSyms, topLevel...)
}

func smallestContainingContainer(containers []rawSym, leaf rawSym) int {
	bestIdx := -1
	bestSize := ^uint32(0)

	for i, c := range containers {
		if leaf.startByte >= c.startByte && leaf.endByte <= c.endByte {
			if size := c.endByte - c.startByte; size < bestSize {
				bestSize = size
				bestIdx = i
			}
		}
	}

	return bestIdx
}
