import '../../features/auth/data/auth_repository.dart';
import '../../features/feed/data/feed_repository.dart';
import '../../features/live/data/live_repository.dart';
import '../../features/video/data/video_repository.dart';
import '../config/app_config.dart';
import '../network/api_client.dart';
import '../session/session_controller.dart';

class AppDependencies {
  AppDependencies({required this.config})
      : apiClient = ApiClient(baseUrl: config.apiBaseUrl),
        sessionController = SessionController() {
    authRepository = AuthRepository(apiClient: apiClient);
    feedRepository = FeedRepository(apiClient: apiClient);
    videoRepository = VideoRepository(apiClient: apiClient);
    liveRepository = LiveRepository(apiClient: apiClient);
  }

  final AppConfig config;
  final ApiClient apiClient;
  final SessionController sessionController;

  late final AuthRepository authRepository;
  late final FeedRepository feedRepository;
  late final VideoRepository videoRepository;
  late final LiveRepository liveRepository;

  void dispose() {
    sessionController.dispose();
    apiClient.close();
  }
}
