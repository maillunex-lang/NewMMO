# 3D migration — Lunex Fantasy 3D (owner decision 2026-11-21)

**Decision:** full real-time 3D (Godot 4.7.2 3D), not prerendered sprites. Owner supplied a Meshy premium account (API key as an environment network secret) and a test phone **iQOO 15R, 12 GB** (flagship class: results prove the *Ultra/High* path; a mid-range device is still needed for the *Low* path guarantee, risk R-DEVICE stays open until then).

## Options considered
| Option | Verdict |
|--------|---------|
| Stay 2D (current) | rejected by the owner: cannot deliver volume, soft shadows, sun rays, free zoom, presets |
| 3D models prerendered to 8-direction sprites | cheaper on phones but fixed view, no dynamic shadows/zoom — rejected |
| **Real-time 3D in Godot** | **adopted** |
| Other engines (Unity/Unreal/Bevy…) | rejected: invariant "keep Godot + Go server"; no reason to leave Godot (3D, GLB import, Android export all supported) |

## Asset source (research, no new code dependency)
Meshy (text/image-to-3D, PBR texturing, auto-rigging, animation library; API `https://api.meshy.ai/openapi/...`). Strong for static props/buildings/trees, acceptable for humanoids, weak for exotic monster rigs. Alternatives kept as fallbacks: CC0 packs (Kenney, Quaternius) and Mixamo animations retargeted — each needs owner OK before vendoring (AGENTS research gate). Meshy credit balance at start: **1454** (monthly 1254 + permanent 200) — budget plan in `docs/research/3d-asset-budget.md`.

## What stays / what changes
Stays: Go server (x,y authoritative), WebSocket protocol, quests/dialogue/inventory/equipment/loot, all `Control` UI scenes, content canon. Changes: world layer (locations, actors, camera, VFX, autopath on the XZ plane, telegraphs, puppets). The 2D layer remains bootable until the **device gate**.

## Device gate (decision to remove 2D)
On the owner's phone, **Low** preset ≥ 55 FPS and **High** ≥ 40 FPS in a scene with 20 puppets + 50 props in view; peak PSS < 1.5 GB, no thermal throttling in 10 min (measure_device.sh). Mid-range reference to be added when available. Until then no claim about FPS is made.

## Slices
S0 foundation (quality presets, camera rig, 3D ground, procedural placeholders bound to server entities) → S1 Meshy hero (mage) + 1 monster + 12 props → S2 Dawn Valley environment kit → S3 all classes/monsters → S4 VFX + sun rays + presets tuning → S5 device gate and 2D removal.
