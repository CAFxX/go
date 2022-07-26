package ssa

// unifytails checks, for each basic block with multiple predecessors,
// if some code in the tail of each predecessor is shared by more than one
// predecessor. If so, it rewrites the common tail from the predecessors,
// places it in a new basic block, and makes the predecessors point to it.
// E.g. given this code:
//
//   foo:
//     x++
//     y++
//     jump done
//   bar:
//     y++
//     jump done
//   baz:
//     x++
//     jump done
//   done:
//     return x, y
//
// it will rewrite it as follows:
//
//   foo:
//     x++
//     jump bb1
//   bar:
//     jump bb1
//   baz:
//     x++
//     jump done
//   done:
//     return x, y
//   bb1:
//     y++
//     jump done
//
// This helps when multiple code paths do similar things (e.g. calling the same function
// with different arguments on each path, returning different values on each path, ...).
func unifytails(f *Func) {
	for changed := true; changed; {
		changed = false
		for _, bb := range f.Blocks {
			if len(bb.Preds) < 2 {
				continue
			}

			m := make(map[valueSig][]Edge)
			for _, pe := range bb.Preds {
				pbb := pe.Block()
				if len(pbb.Succs) != 1 {
					// The predecessor has multiple successors. We can't sink the last
					// value, as we would need to sink it in all successors.
					// TODO: Can we relax this restriction?
					continue
				}
				lastVal := pbb.Values[len(pbb.Values)-1]
				lastValSig, ok := getValueSig(lastVal)
				if ok {
					m[lastValSig] = append(m[lastValSig], pe)
				}
			}
			for _, preds := range m {
				if len(preds) < 2 {
					continue
				}
				nbb := f.NewBlock()
			}
		}
	}
}
