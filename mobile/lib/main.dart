import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'constants/app_theme.dart';
import 'database/db_helper.dart';
import 'services/sync_service.dart';
import 'ui/screens/main_navigation_shell.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Edge-to-edge transparent system bars for modern Android (OnePlus 11 5G)
  SystemChrome.setSystemUIOverlayStyle(
    const SystemUiOverlayStyle(
      statusBarColor: Colors.transparent,
      statusBarIconBrightness: Brightness.light,
      systemNavigationBarColor: Colors.transparent,
      systemNavigationBarIconBrightness: Brightness.light,
    ),
  );

  runApp(const ImportManagerMobileApp());

  // Background non-blocking warm up
  DBHelper.instance.database.ignore();
  SyncService.instance.init().ignore();
}

class ImportManagerMobileApp extends StatelessWidget {
  const ImportManagerMobileApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Import Tracker',
      debugShowCheckedModeBanner: false,
      themeMode: ThemeMode.dark, // Default to Dark Mode for OnePlus 11 AMOLED
      darkTheme: AppTheme.darkTheme,
      theme: AppTheme.lightTheme,
      home: const MainNavigationShell(),
    );
  }
}
