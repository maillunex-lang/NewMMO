# Meshy credit budget (start 1454 credits)

Costs are read from API responses/balance deltas during S1 and recorded here (not assumed). Rule: never spend more than 15% of the remaining balance in one batch without recording the result; generate previews first, refine only chosen ones.

| Batch | Plan | Credits spent (actual) | Balance after |
|-------|------|-----------------------:|--------------:|
| start | — | 0 | 1454 |
| S1 batch 1: 11 previews (20 each) + 11 refines (10 each, observed total drop 350) | tree_oak, tree_pine, rock, cottage, fence, lantern, crystal, mage hero, bandit, slime-like creature, wolf | 350 | 1104 |

Observed unit costs: preview 20, refine 10 (balance delta slightly above the sum of reported costs). Raw GLBs are 5–8 MB each (4K textures) → shrunk to 0.1–1.2 MB with `tools/meshy/optimize_glb.py`; 11 models total 5.4 MB.
| S2 batch 2: 17 previews (340) + 17 refines (170) | 4 more heroes, mage (T-pose redo), 3 NPCs, goblin, bush, flowers, mushrooms, stump, birch, well, barrel, haystack | ~510 | 574 |
| rigging 10 × 5 + animations 5 × 7 (idle, attack, cast, hit, dead) | all humanoids rigged; one shared animation set | ~45 + 35 | 529 → see balance |

Rigging notes: auto-rig failed ("Pose estimation failed") on a robed, staff-holding mage — humanoids must be generated in a strict T/A-pose without long skirts; works for all 10 after that. Rig basic walk/run come free with the rig. Animations are retargeted to every humanoid because Meshy's rig has the same 24 bones.
| S4 (owner: spend everything): 9 objects/enemies (previews 180 + refines 90 + 3 rigs 15), 14 UI/FX images (42), 24 more skill icons + 6 earlier (3 each), icon test | UI art, FX textures, remaining icons, world objects, dungeon enemies | all remaining | **19** |

The owner's order was UI → world objects → FX/skills → NPC/enemies; the last 19 credits are kept as a reserve.

