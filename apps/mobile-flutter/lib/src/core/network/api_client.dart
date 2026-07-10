import 'dart:convert';
import 'dart:io';

import 'api_exception.dart';

class ApiClient {
  ApiClient({required this.baseUrl});

  final String baseUrl;
  final HttpClient _client = HttpClient();

  void close({bool force = false}) {
    _client.close(force: force);
  }

  Future<Map<String, dynamic>> get(
    String path, {
    String? token,
  }) {
    return request('GET', path, token: token);
  }

  Future<Map<String, dynamic>> post(
    String path, {
    Map<String, dynamic>? body,
    String? token,
    String? idempotencyKey,
  }) {
    return request(
      'POST',
      path,
      body: body,
      token: token,
      idempotencyKey: idempotencyKey,
    );
  }

  Future<Map<String, dynamic>> request(
    String method,
    String path, {
    Map<String, dynamic>? body,
    String? token,
    String? idempotencyKey,
  }) async {
    final uri = Uri.parse('$baseUrl$path');
    final request = await _client.openUrl(method, uri);
    request.headers.set(HttpHeaders.acceptHeader, 'application/json');
    if (token != null && token.isNotEmpty) {
      request.headers.set(HttpHeaders.authorizationHeader, 'Bearer $token');
    }
    if (idempotencyKey != null && idempotencyKey.isNotEmpty) {
      request.headers.set('Idempotency-Key', idempotencyKey);
    }
    if (body != null) {
      request.headers.contentType = ContentType.json;
      request.write(jsonEncode(body));
    }

    final response = await request.close();
    final text = await response.transform(utf8.decoder).join();
    final decoded = text.isEmpty ? <String, dynamic>{} : jsonDecode(text);
    final payload = decoded is Map<String, dynamic> ? decoded : <String, dynamic>{};

    if (response.statusCode < 200 || response.statusCode >= 300) {
      final error = payload['error'] as Map<String, dynamic>?;
      throw ApiException(
        error?['message'] as String? ?? response.reasonPhrase,
        response.statusCode,
      );
    }

    return payload;
  }
}
