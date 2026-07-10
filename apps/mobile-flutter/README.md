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

Install Flutter, then run:

```bash
cd apps/mobile-flutter
flutter pub get
flutter run --dart-define=API_BASE_URL=http://localhost:8080
```

For Android emulator, use the host loopback address:

```bash
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080
```

## Current App Shell

- `Auth`: login and local in-memory session.
- `Feed`: ready videos from `/api/v1/feed`.
- `Upload`: create upload requests through `/api/v1/videos/upload-requests`.
- `Live`: live session list from `/api/v1/live-sessions`.
- `Profile`: session and API configuration.

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

Platform folders (`android/`, `ios/`, etc.) can be generated later with Flutter tooling if needed.
