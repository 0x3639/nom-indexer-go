import 'dart:async';

import 'package:znn_sdk_dart/znn_sdk_dart.dart';
import 'config/config.dart';
import 'services/database_service.dart';
import 'indexer/nom_indexer.dart';

Future<void> main(List<String> arguments) async {
  Config.load();

  await DatabaseService().initialize();

  final node = Zenon();
  await node.wsClient.initialize(Config.nodeUrlWs);

  final indexer = NomIndexer(node);
  await indexer.sync();
  await indexer.updatePillarVotingActivity();
  await indexer.updateTokenHolderCounts();

  _runIndexer(indexer);
  _runCron(indexer);
}

_runIndexer(NomIndexer indexer) {
  // NOTE: Use polling for now until issues with the unreliable WS subscription are resolved.
  Timer.periodic(Duration(seconds: 10), (Timer t) async {
    t.cancel();
    await indexer.sync();
    _runIndexer(indexer);
  });
}

_runCron(NomIndexer indexer) {
  Timer.periodic(Duration(minutes: 10), (Timer t) async {
    t.cancel();
    final stopwatch = Stopwatch()..start();
    await indexer.updatePillarVotingActivity();
    await indexer.updateTokenHolderCounts();
    print('runCron() executed in ${stopwatch.elapsed.inMilliseconds} msecs');
    stopwatch.stop();
    _runCron(indexer);
  });
}
