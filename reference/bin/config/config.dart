import 'dart:io';

import 'package:settings_yaml/settings_yaml.dart';

class Config {
  static String _nodeUrlWs = 'ws://127.0.0.1:35998';

  static String _databaseAddress = '127.0.0.1';
  static int _databasePort = 5432;
  static String _databaseName = '';
  static String _databaseUsername = '';
  static String _databasePassword = '';

  static String get nodeUrlWs {
    return _nodeUrlWs;
  }

  static String get databaseAddress {
    return _databaseAddress;
  }

  static int get databasePort {
    return _databasePort;
  }

  static String get databaseName {
    return _databaseName;
  }

  static String get databaseUsername {
    return _databaseUsername;
  }

  static String get databasePassword {
    return _databasePassword;
  }

  static void load() {
    // Check environment variables first
    final envNodeUrlWs = Platform.environment['NODE_URL_WS'];
    final envDatabaseAddress = Platform.environment['DATABASE_ADDRESS'];
    final envDatabasePort = Platform.environment['DATABASE_PORT'];
    final envDatabaseName = Platform.environment['DATABASE_NAME'];
    final envDatabaseUsername = Platform.environment['DATABASE_USERNAME'];
    final envDatabasePassword = Platform.environment['DATABASE_PASSWORD'];

    // If all required environment variables are set, use them
    if (envNodeUrlWs != null &&
        envDatabaseAddress != null &&
        envDatabasePort != null &&
        envDatabaseName != null &&
        envDatabaseUsername != null &&
        envDatabasePassword != null) {
      _nodeUrlWs = envNodeUrlWs;
      _databaseAddress = envDatabaseAddress;
      _databasePort = int.parse(envDatabasePort);
      _databaseName = envDatabaseName;
      _databaseUsername = envDatabaseUsername;
      _databasePassword = envDatabasePassword;
      
      print('Configuration loaded from environment variables');
      return;
    }

    // Otherwise, fall back to config.yaml
    try {
      final settings = SettingsYaml.load(
          pathToSettings: '${Directory.current.path}/config.yaml');

      _nodeUrlWs = settings['node_url_ws'] as String;

      _databaseAddress = settings['database_address'] as String;
      _databasePort = settings['database_port'] as int;
      _databaseName = settings['database_name'] as String;
      _databaseUsername = settings['database_username'] as String;
      _databasePassword = settings['database_password'] as String;
      
      print('Configuration loaded from config.yaml');
    } catch (e) {
      print('Warning: Could not load config.yaml and no complete environment variables found');
      print('Using default values where possible');
      
      // Use individual environment variables if available, otherwise keep defaults
      _nodeUrlWs = envNodeUrlWs ?? _nodeUrlWs;
      _databaseAddress = envDatabaseAddress ?? _databaseAddress;
      _databasePort = envDatabasePort != null ? int.parse(envDatabasePort) : _databasePort;
      _databaseName = envDatabaseName ?? _databaseName;
      _databaseUsername = envDatabaseUsername ?? _databaseUsername;
      _databasePassword = envDatabasePassword ?? _databasePassword;
    }
  }
}
