import 'package:flutter/material.dart';

import 'core/config/app_config.dart';
import 'core/di/app_dependencies.dart';
import 'navigation/app_shell.dart';
import 'shared/theme/app_theme.dart';

class AiopsVideoMobileApp extends StatefulWidget {
  const AiopsVideoMobileApp({super.key});

  @override
  State<AiopsVideoMobileApp> createState() => _AiopsVideoMobileAppState();
}

class _AiopsVideoMobileAppState extends State<AiopsVideoMobileApp> {
  late final AppDependencies _dependencies = AppDependencies(
    config: AppConfig.fromEnvironment(),
  );

  @override
  void initState() {
    super.initState();
    _dependencies.sessionController.restore();
  }

  @override
  void dispose() {
    _dependencies.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'AIOps Video',
      debugShowCheckedModeBanner: false,
      theme: buildAppTheme(),
      home: AppShell(dependencies: _dependencies),
    );
  }
}
