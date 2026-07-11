import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;

import 'api_exception.dart';

class ApiClient {
  ApiClient({
    required this.baseUrl,
    http.Client? httpClient,
  }) : _client = httpClient ?? http.Client();

  final String baseUrl;
  final http.Client _client;

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

  Future<void> putBytes(
    Uri uri, {
    required Uint8List bytes,
    required String contentType,
  }) async {
    final response = await _client.put(
      uri,
      headers: {'Content-Type': contentType},
      body: bytes,
    );
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw ApiException(
        response.reasonPhrase ?? 'Object upload failed',
        response.statusCode,
      );
    }
  }

  Future<Map<String, dynamic>> request(
    String method,
    String path, {
    Map<String, dynamic>? body,
    String? token,
    String? idempotencyKey,
  }) async {
    final uri = Uri.parse('$baseUrl$path');
    final headers = <String, String>{
      'Accept': 'application/json',
      if (token != null && token.isNotEmpty) 'Authorization': 'Bearer $token',
      if (idempotencyKey != null && idempotencyKey.isNotEmpty) 'Idempotency-Key': idempotencyKey,
    };

    late final http.Response response;
    switch (method) {
      case 'GET':
        response = await _client.get(uri, headers: headers);
        break;
      case 'POST':
        response = await _client.post(
          uri,
          headers: {
            ...headers,
            if (body != null) 'Content-Type': 'application/json',
          },
          body: body == null ? null : jsonEncode(body),
        );
        break;
      default:
        throw ArgumentError.value(method, 'method', 'Unsupported HTTP method');
    }

    final payload = _decode(response.body);
    if (response.statusCode < 200 || response.statusCode >= 300) {
      final error = payload['error'] as Map<String, dynamic>?;
      throw ApiException(
        error?['message'] as String? ?? response.reasonPhrase ?? 'Request failed',
        response.statusCode,
      );
    }

    return payload;
  }

  void close() {
    _client.close();
  }

  Map<String, dynamic> _decode(String text) {
    if (text.isEmpty) {
      return <String, dynamic>{};
    }
    final decoded = jsonDecode(text);
    return decoded is Map<String, dynamic> ? decoded : <String, dynamic>{};
  }
}
