# mobile-flutter

Flutter mobile app demo for the video platform.

## Scope

- Login.
- Browse viewer feed.
- Create video upload intent and track processing status.
- Browse live sessions.
- Keep AIOps logic out of the mobile app.

The mobile app represents the end-user product experience. Admin and RCA workflows stay in `apps/admin-web`.

## Development

Install Flutter `3.38.4+`, then run:

```bash
cd apps/mobile-flutter
flutter pub get
flutter analyze
flutter test
flutter run --dart-define=API_BASE_URL=http://localhost:8080
```

Preview in Chrome while iOS Simulator runtime is not installed:

```bash
flutter run -d chrome --dart-define=API_BASE_URL=http://localhost:8080
```

For Android emulator, use the host loopback address:

```bash
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080
```

## Current App Shell

- `Auth`: login and local in-memory session.
- `Session`: access token persistence through platform secure storage.
- `Feed`: ready videos from `/api/v1/feed`.
- `Upload`: pick a video, calculate SHA-256, create upload request, upload to the presigned URL, and confirm upload.
- `Live`: live session list from `/api/v1/live-sessions`.
- `Profile`: session and API configuration.
- `iOS`: native project files are generated under `ios/`.
- `Web`: preview project files are generated under `web/` for fast visual checks.

## Architecture

The app uses a feature-first structure so it can grow without turning screens into API scripts.

```text
lib/
  main.dart
  src/
    app.dart
    core/
      config/              # environment config
      di/                  # dependency container
      network/             # HTTP client and API errors
      session/             # app-wide session controller
    features/
      auth/
        data/              # repositories / remote data access
        domain/            # entities
        presentation/      # screens/sheets/widgets
      feed/
      live/
      profile/
      video/
    navigation/            # app shell and tab navigation
    shared/
      theme/
      widgets/
```

Rules for future work:

- Keep API calls in `data/*_repository.dart`.
- Keep serializable app models in `domain/`.
- Keep Flutter widgets in `presentation/` or `shared/widgets/`.
- Keep app-wide wiring in `core/di/app_dependencies.dart`.
- Do not place AIOps RCA logic in mobile; mobile is the end-user demo client.

Additional platform folders such as `android/` can be generated later with Flutter tooling if needed.

## iOS Notes

The iOS project is generated and ready for Simulator once an iOS runtime is installed in Xcode.

```bash
open ios/Runner.xcworkspace
flutter build ios --simulator
```

If Xcode reports that an iOS platform is missing, install the runtime from Xcode Components/Platforms or run:

```bash
sudo xcodebuild -downloadPlatform iOS
```

Building for a physical device requires selecting a Development Team in Xcode under `Runner > Signing & Capabilities`.

If the local machine does not have enough disk space for the iOS Simulator runtime, use the Chrome preview command above until space is available.

## Verification

Current checks:

```bash
flutter analyze
flutter test
flutter build web --dart-define=API_BASE_URL=http://localhost:8080
```

`flutter build ios --simulator` currently requires an installed iOS Simulator runtime in Xcode.
