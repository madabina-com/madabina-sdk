<?php

$root = dirname(__DIR__, 3);
$autoloadPath = is_file(dirname(__DIR__).'/vendor/autoload.php')
    ? dirname(__DIR__).'/vendor/autoload.php'
    : $root.'/web-app/vendor/autoload.php';
$autoload = require $autoloadPath;

if (!class_exists(\Madabina\Sdk\MadabinaClient::class)) {
    require $root.'/sdk/php/src/MadabinaClient.php';
}

$autoload->addPsr4('Madabina\\Laravel\\', dirname(__DIR__).'/src');
require dirname(__DIR__).'/src/MadabinaServiceProvider.php';
require dirname(__DIR__).'/src/Facades/Madabina.php';
