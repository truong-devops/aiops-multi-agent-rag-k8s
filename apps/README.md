# Apps

Thư mục này chứa các ứng dụng client của product demo.

- `admin-web`: Next.js operations dashboard cho admin/SRE. App dùng API gateway để theo dõi video pipeline, feed, livestream, service readiness, incident queue và AIOps RCA readiness.
- `mobile-flutter`: Flutter end-user app shell cho login, feed, upload video, trạng thái video và live sessions.

Admin web là trung tâm demo vận hành và AIOps. Mobile app phục vụ trải nghiệm người dùng cuối kiểu video/livestream, không chứa logic RCA.

## Run Locally

Admin web:

```bash
cd apps/admin-web
nvm use
npm install
npm run dev
```

Mobile Flutter:

```bash
cd apps/mobile-flutter
flutter pub get
flutter run -d chrome --dart-define=API_BASE_URL=http://localhost:8080
```
