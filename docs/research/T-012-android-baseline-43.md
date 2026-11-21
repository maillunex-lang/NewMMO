# T-012 — Android debug APK baseline (Godot **4.3**)

**Date:** 2026-11-21 · **Lane:** client · **Extra-Write:** `tools/android/**`  
**Engine pin:** Godot **4.3.stable** (`77dcf97d8`) — **not** 4.7.2

## 1. Need

Reproduce a signed debug APK for `com.lunex.newmmo` (prebuilt, `gradle_build/use_gradle_build=false`), verify package/ABI/SDK levels/JSON packing, leave a 4.3 measurement baseline for later 4.7.2 comparison.

## 2. Sources

| Source | Used |
|--------|------|
| Godot 4.3 docs | [Exporting for Android](https://docs.godotengine.org/en/4.3/tutorials/export/exporting_for_android.html) |
| Godot release | [4.3-stable](https://github.com/godotengine/godot/releases/tag/4.3-stable) + `SHA512-SUMS.txt` |
| Adoptium | Temurin 17 Linux x64 + published SHA-256 |
| Google | Android cmdline-tools zip + SHA-256 on developer.android.com/studio |
| Hysen / GitHub | [godot-ci](https://hysenlabs.com/projects/abarichello-godot-ci), [godot-export](https://hysenlabs.com/projects/firebelley-godot-export) |
| In-repo | `godot-4-7-2-upgrade.md` §5–§6, `export_presets.cfg`, AUDIT B-APK |

## 3. Candidates (≥3)

| # | Approach | Pros | Cons | Verdict |
|---|----------|------|------|---------|
| 1 | **Manual: Temurin 17 + SDK cmdline-tools + sdkmanager + official templates** in `~/` | Minimal; SHA-verified; matches owner approval | Manual license accept | **Adopt** |
| 2 | [abarichello/godot-ci](https://hysenlabs.com/projects/abarichello-godot-ci) Docker | Reproducible CI image | Docker image pull; not needed on this VM for baseline | Research only — **not used** |
| 3 | Android Studio Iguana+ | Docs-supported one-shot SDK | **Forbidden** by task (Studio/emulators) | Reject |
| 4 | firebelley/godot-export Action | CI export automation | Needs GH Actions + templates download in CI | Future director/CI |

## 4. Godot 4.3 required packages (export)

From 4.3 docs:

- **OpenJDK 17**
- Android SDK: `platform-tools`, `build-tools;34.0.0`, `platforms;android-34`, `cmdline-tools;latest`
- Docs’ full `sdkmanager` line also lists `cmake;3.10.2.4988404` and `ndk;23.2.8568313`

**Policy for this task:** preset uses **prebuilt** APK (`use_gradle_build=false`). Install **without NDK/CMake/Gradle first**. If export fails requiring them, stop and report (do not expand silently). Export templates: `Godot_v4.3-stable_export_templates.tpz` → `~/.local/share/godot/export_templates/4.3.stable/`.

## 5. Installed artifacts (this run)

| Component | Version | Source | Official checksum | Local path |
|-----------|---------|--------|-------------------|------------|
| Temurin JDK | 17.0.21.1+2 | Adoptium GitHub | SHA-256 `0b0ac5eab6db6abaec68c9913bcddb31321e5da411c45073d520d9c125e6f7b4` ✓ | `~/jdk/jdk-17.0.21.1+2` |
| Android cmdline-tools | rev `15859902` | dl.google.com | SHA-256 `4e4c464f145a7512b57d088ac6c278c03c9eea610886b35a5e0804e74eedf583` ✓ | `~/android-sdk/cmdline-tools/latest` |
| SDK packages | platform-tools, build-tools;34.0.0, platforms;android-34 | sdkmanager | (Google package manager) | `~/android-sdk` |
| Export templates | 4.3-stable | godotengine/godot releases | SHA-512 `476366caf0…6406d6a` ✓; local SHA-256 `f5f10dbaf56a7908349e64b94ea8c2c93c2cc11fb1014c9565b68a2bb437b922` | `~/.local/share/godot/export_templates/4.3.stable` |
| Debug keystore | local | `keytool` | n/a (secret) | **outside git** `~/lunex-android/debug.keystore` |

**NDK/CMake:** not installed. Prebuilt APK export succeeded without them (docs list them for “complete” sdkmanager; not required for this preset).

## 6. Reproduce (high level)

```sh
export JAVA_HOME="$HOME/jdk/jdk-17.0.21.1+2"
export ANDROID_HOME="$HOME/android-sdk"
export PATH="$JAVA_HOME/bin:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$ANDROID_HOME/build-tools/34.0.0:$PATH"
export GODOT_ANDROID_KEYSTORE_DEBUG_PATH="$HOME/lunex-android/debug.keystore"
export GODOT_ANDROID_KEYSTORE_DEBUG_USER=androiddebugkey
export GODOT_ANDROID_KEYSTORE_DEBUG_PASSWORD='…'   # not in git
./tools/android/build_debug_apk.sh
```

## 7. APK verification (this run)

| Field | Value |
|-------|-------|
| godot | `4.3.stable.official.77dcf97d8` |
| git SHA (at build) | see branch tip |
| APK | `client/build/LunexMMORPG-debug.apk` (gitignored) |
| size | **24686211** bytes (~23.5 MiB) |
| package | `com.lunex.newmmo` |
| versionName | `0.0.1` |
| minSdk | **21** |
| targetSdk | **34** |
| ABI | **arm64-v8a** |
| apksigner | verify OK (CN=Android Debug) |
| JSON | `assets/data/quests/mq_01_01_to_05.json`, `assets/data/dialogues/mq_01_01_to_05.json` |

## 8. Device metrics

`adb` present after SDK install; **no phone attached** (`adb devices` empty) → **устройство недоступно**. Owner protocol: install APK then `./tools/android/measure_device.sh [apk]` (3× cold start + gfxinfo/meminfo; optional `MEASURE_WAIT_SEC=300`). **No invented FPS/PSS.**

## 9. Baseline pin for later 4.7.2 compare

- Engine: `4.3.stable.official.77dcf97d8`
- Graphics: GL Compatibility (project.godot)
- Viewport: 1280×720 landscape
- Device: *unavailable this run*
