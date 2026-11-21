# In-game background asset download (3D)

**Why:** the APK stays small; heavy models/animations (~25 MB now, more later) are downloaded after install while the player logs in.

**Client:** autoload `AssetPacks` (`client/scripts/core/asset_pack.gd`): fetches `<base>/manifest.json` (`{version, packs:[{name,file,sha256,size}]}`), downloads missing `.pck` files over HTTPS into `user://packs/` (checksum-verified, atomic rename, retry with back-off, resumes after restart because the cache is content-addressed by file name), mounts them with `ProjectSettings.load_resource_pack`, shows «Загрузка ресурсов… N%» and emits `packs_ready`; `View3D` then swaps its placeholders for the real models. Desktop/editor runs that already have `res://assets/models` skip it. Base URL: `NEWMMO_ASSETS_URL` (default `https://rikagro.ru/newmmo/assets/`).

**Build/host:** `tools/android/build_asset_packs.sh` → `build/packs/world3d-<sha8>.pck` + `manifest.json`; upload the folder to the VPS path served at `/newmmo/assets/` (static files, not part of Git; **not done here — no VPS access**). The Android export preset excludes `assets/models/*` and `assets/anims/*`.

**Verified:** `ASSETS_SMOKE` (local static server): first run downloads 22 MB and mounts it, second run uses the cache (0 bytes). **Not verified:** real VPS hosting, Android runtime mount of imported GLB scenes inside a .pck, mobile network behaviour.
