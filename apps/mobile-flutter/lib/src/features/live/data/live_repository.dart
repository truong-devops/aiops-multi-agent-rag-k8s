import '../../../core/network/api_client.dart';
import '../domain/live_session.dart';

class LiveRepository {
  const LiveRepository({required ApiClient apiClient}) : _apiClient = apiClient;

  final ApiClient _apiClient;

  Future<List<LiveSession>> listLiveSessions({
    required String token,
    int limit = 20,
  }) async {
    final body = await _apiClient.get(
      '/api/v1/live-sessions?limit=$limit',
      token: token,
    );
    final items = body['data'] as List<dynamic>? ?? const [];
    return items
        .map((item) => LiveSession.fromJson(item as Map<String, dynamic>))
        .toList();
  }
}
