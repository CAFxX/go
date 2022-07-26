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
	m := make(map[valueSig][]Edge)

	for changed := true; changed; {
		changed = false
		for _, bb := range f.Blocks {
			if len(bb.Preds) < 2 {
				continue
			}

			for k := range m {
				delete(m, k)
			}

			for _, pe := range bb.Preds {
				pbb := pe.Block()
				if len(pbb.Succs) != 1 || pbb.Kind != BlockPlain {
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
				nbb := f.NewBlock(BlockPlain)
				// TODO: in nbb, add Phis for the the Values used by the Value
				// TODO: in nbb, add the Value
				nbb.AddEdgeTo(bb)
				// TODO: in bb, update the P
				for _, pred := range preds {
					updateEdge(pred, nbb)
				}
			}
		}
	}
}

func getValueSig(v *Value) (valueSig, bool) {
	return "", false
}

type valueSig string
