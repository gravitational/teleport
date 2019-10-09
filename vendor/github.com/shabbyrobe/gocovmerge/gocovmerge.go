// Package gocovmerge takes the results from multiple `go test -coverprofile`
// runs and merges them into one profile
package gocovmerge

import (
	"fmt"
	"io"
	"sort"

	"golang.org/x/tools/cover"
)

func AddProfile(profiles []*cover.Profile, p *cover.Profile) []*cover.Profile {
	i := sort.Search(len(profiles), func(i int) bool { return profiles[i].FileName >= p.FileName })
	if i < len(profiles) && profiles[i].FileName == p.FileName {
		MergeProfiles(profiles[i], p)
	} else {
		profiles = append(profiles, nil)
		copy(profiles[i+1:], profiles[i:])
		profiles[i] = p
	}
	return profiles
}

func DumpProfiles(profiles []*cover.Profile, out io.Writer) error {
	if len(profiles) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(out, "mode: %s\n", profiles[0].Mode); err != nil {
		return err
	}
	for _, p := range profiles {
		for _, b := range p.Blocks {
			if _, err := fmt.Fprintf(out, "%s:%d.%d,%d.%d %d %d\n", p.FileName, b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.NumStmt, b.Count); err != nil {
				return err
			}
		}
	}
	return nil
}

func MergeProfiles(into *cover.Profile, merge *cover.Profile) error {
	if into.Mode != merge.Mode {
		return fmt.Errorf("cannot merge profiles with different modes")
	}
	// Since the blocks are sorted, we can keep track of where the last block
	// was inserted and only look at the blocks after that as targets for merge
	startIndex := 0
	for _, b := range merge.Blocks {
		var err error
		startIndex, err = mergeProfileBlock(into, b, startIndex)
		if err != nil {
			return err
		}
	}
	return nil
}

func mergeProfileBlock(p *cover.Profile, pb cover.ProfileBlock, startIndex int) (int, error) {
	sortFunc := func(i int) bool {
		pi := p.Blocks[i+startIndex]
		return pi.StartLine >= pb.StartLine && (pi.StartLine != pb.StartLine || pi.StartCol >= pb.StartCol)
	}

	i := 0
	if sortFunc(i) != true {
		i = sort.Search(len(p.Blocks)-startIndex, sortFunc)
	}

	i += startIndex
	if i < len(p.Blocks) && p.Blocks[i].StartLine == pb.StartLine && p.Blocks[i].StartCol == pb.StartCol {
		if p.Blocks[i].EndLine != pb.EndLine || p.Blocks[i].EndCol != pb.EndCol {
			return i, fmt.Errorf("gocovmerge: overlapping merge %v %v %v", p.FileName, p.Blocks[i], pb)
		}
		switch p.Mode {
		case "set":
			p.Blocks[i].Count |= pb.Count
		case "count", "atomic":
			p.Blocks[i].Count += pb.Count
		default:
			return i, fmt.Errorf("gocovmerge: unsupported covermode '%s'", p.Mode)
		}

	} else {
		if i > 0 {
			pa := p.Blocks[i-1]
			if pa.EndLine >= pb.EndLine && (pa.EndLine != pb.EndLine || pa.EndCol > pb.EndCol) {
				return i, fmt.Errorf("gocovmerge: overlap before %v %v %v", p.FileName, pa, pb)
			}
		}
		if i < len(p.Blocks)-1 {
			pa := p.Blocks[i+1]
			if pa.StartLine <= pb.StartLine && (pa.StartLine != pb.StartLine || pa.StartCol < pb.StartCol) {
				return i, fmt.Errorf("gocovmerge: overlap after %v %v %v", p.FileName, pa, pb)
			}
		}
		p.Blocks = append(p.Blocks, cover.ProfileBlock{})
		copy(p.Blocks[i+1:], p.Blocks[i:])
		p.Blocks[i] = pb
	}

	return i + 1, nil
}
