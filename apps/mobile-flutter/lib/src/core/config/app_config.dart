class AppConfig {
  const AppConfig({
    required this.apiBaseUrl,
    required this.appName,
  });

  factory AppConfig.fromEnvironment() {
    return const AppConfig(
      apiBaseUrl: String.fromEnvironment(
        'API_BASE_URL',
        defaultValue: 'http://localhost:8080',
      ),
      appName: 'AIOps Video',
    );
  }

  final String apiBaseUrl;
  final String appName;
}
