# Lunex MMORPG - approved decisions

This file distinguishes approved requirements from provisional design. The full 42-screen registry and campaign details are design targets, not all release-one commitments.

## Core
- Landscape Android fantasy MMORPG; single connected continent with region channels.
- Isometric 2.5D world; animated prerendered directional 2D sprites. No real-time 3D character rendering.
- COMBAT-002 C: auto-combat with manual override; autopath and auto-loot.
- WORLD-002 A: one connected fantasy continent.
- QUEST-001 B: story NPC quests and dialogues; important dialogue pauses automation.
- PROGRESS-001 A: classic levels, gear and skills.
- LEVEL-001 B: cap 60.
- CLASS-001 C: five base classes and specializations available from start: Guardian, Berserker, Mage, Ranger, Oracle, two specializations each.
- PET-002 B: one active pet.
- TRANSFORM-001 B: awakening at level 40.
- SKILL-001 B: 6 equipped active skills out of 12 per class.
- AUTO-001 C: configurable auto-combat priorities.
- GEAR-001 C: sockets, enchantment, reforging, awakening.
- BALANCE-001 B: all classes can solo story; group roles matter.

## Narrative
- STORY-001 B: epic fantasy with occasional sharp satire.
- DIALOGUE-001 B: substantial portrait dialogue and response choices.
- NPC-001 B: key NPC stories and relationships.
- DUNGEON-001 B: solo and group dungeons.
- CHAPTER-001 C: chapter I contains 20 main quests.
- DIALOGUE-002 B: choices affect NPC reactions, not main plot access.
- BOSS-001 B: auto-combat works but manual play is advantageous.
- ROUTE-001 B: autopath pauses at important interactions.
- Campaign names, locations and NPC names remain working proposals until specifically approved.

## World - GDD-07
- CITY-001 B: living towns with trade and social spaces.
- NPC-WORLD-001 B: NPC routes, schedules, reactions.
- MONSTER-001 B: patrols, elites, bosses, events.
- TRAVEL-001 B: walk, autopath, teleport and mounts.

## Social - GDD-08
- PARTY-001 C: groups of up to 5 with role-aware matchmaking.
- GUILD-001 C: territorial ownership and diplomacy.
- PVP-001 C: mostly open PvP with crime status.
- EVENT-001 B: dynamic invasions, caravans, bosses.

## Economy - GDD-09
- ECON-001 B: free trade, auction, direct trades, player stalls.
- CRAFT-001 B: professions, recipes, mastery and rare materials.
- DEATH-001 B: repair costs and some resource loss in dangerous zones.
- MONET-001 C: VIP, accelerators, bundles and paid conveniences. Specific bonuses, caps and PvP/economic balance remain to be designed.

## UI - GDD-10/11
- HUD-001 B: classic MMO HUD.
- CTRL-001 B: joystick, autopath, auto-combat, manual skills.
- UI-001 C: Premium Stylized Fantasy, dark panels and gold accents.
- CUSTOM-001 C: move, scale, fade and save HUD profiles.
- NAV-001 B: Hero / World / Menu.
- SCREENS-001 B: phased full registry of 42 screens/panels.
- ADAPTIVE-001 C: responsive layout plus manual profiles.
- DESIGN-001 B: modern premium fantasy with restrained decoration.
- UI-EDITOR-001 C: Godot visual editing of major UI via Control-based .tscn scenes, reusable components, Theme .tres; scripts implement behavior, not entire screen geometry.

## Unresolved before production-scale launch
Exact open-PvP crime and loot rules; VIP limits; anti-bot and economy sinks; zone population/load targets; combat tick and replication; asset art direction and sprite atlas budget; security, privacy and payment compliance.

## World editor
- WORLD-EDITOR-001 C: visually editable location scenes, reusable props and editor-side placement; server map data must be validated separately. See docs/WORLD_EDITOR.md.
