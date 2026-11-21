# Meshy credit budget (start 1454 credits)

Costs are read from API responses/balance deltas during S1 and recorded here (not assumed). Rule: never spend more than 15% of the remaining balance in one batch without recording the result; generate previews first, refine only chosen ones.

| Batch | Plan | Credits spent (actual) | Balance after |
|-------|------|-----------------------:|--------------:|
| start | — | 0 | 1454 |
| S1 batch 1: 11 previews (20 each) + 11 refines (10 each, observed total drop 350) | tree_oak, tree_pine, rock, cottage, fence, lantern, crystal, mage hero, bandit, slime-like creature, wolf | 350 | 1104 |

Observed unit costs: preview 20, refine 10 (balance delta slightly above the sum of reported costs). Raw GLBs are 5–8 MB each (4K textures) → shrunk to 0.1–1.2 MB with `tools/meshy/optimize_glb.py`; 11 models total 5.4 MB.

