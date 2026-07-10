import 'package:flutter/material.dart';

import '../../../core/config/app_config.dart';
import '../../../core/session/session_controller.dart';
import '../../../shared/widgets/app_section.dart';

class ProfileScreen extends StatelessWidget {
  const ProfileScreen({
    super.key,
    required this.config,
    required this.sessionController,
  });

  final AppConfig config;
  final SessionController sessionController;

  @override
  Widget build(BuildContext context) {
    final user = sessionController.session?.user;
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        const AppSectionHeader(
          title: 'Profile',
          subtitle: 'Mobile client session.',
        ),
        const SizedBox(height: 12),
        AppSection(
          title: user?.displayName.isNotEmpty == true ? user!.displayName : 'Guest',
          subtitle: user?.email ?? 'Not signed in',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _InfoRow(label: 'API', value: config.apiBaseUrl),
              _InfoRow(label: 'User ID', value: user?.id ?? '-'),
              _InfoRow(label: 'Roles', value: user?.roles.join(', ') ?? '-'),
              const SizedBox(height: 12),
              SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: sessionController.isSignedIn ? sessionController.clear : null,
                  icon: const Icon(Icons.logout),
                  label: const Text('Sign out'),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 76,
            child: Text(label, style: const TextStyle(color: Color(0xFF6B7280))),
          ),
          Expanded(child: Text(value)),
        ],
      ),
    );
  }
}
