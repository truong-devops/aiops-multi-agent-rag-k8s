import '../../../core/network/api_client.dart';
import '../domain/session.dart';

class AuthRepository {
  const AuthRepository({required ApiClient apiClient}) : _apiClient = apiClient;

  final ApiClient _apiClient;

  Future<Session> login({
    required String email,
    required String password,
  }) async {
    final body = await _apiClient.post(
      '/api/v1/auth/login',
      body: {
        'email': email,
        'password': password,
      },
    );
    return Session.fromJson(body['data'] as Map<String, dynamic>);
  }
}
