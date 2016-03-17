/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

var gulp = require("gulp");
var webpack = require('webpack');
var webpackConfig = require('./webpack.config.js');
var gutil = require('gulp-util');
var shell = require('gulp-shell');
var exec = require('child_process').execSync;
var fs = require('fs');

var VERSION = '0.1';
var BUILD_OUTPUT = 'dist';
var BUILD_OUTPUT_ASSETS = BUILD_OUTPUT+'/app/assets';

gulp.task('default', ['clean', 'copy:assets', 'copy:html', 'webpack:build']);

gulp.task('clean', function(){
  exec('rm -rf "' + BUILD_OUTPUT + '"');
  exec('mkdir  "' + BUILD_OUTPUT + '"');
});

gulp.task('copy:html', function(){
  var indexHtml  = fs.readFileSync('src/index.html', 'utf8');
  indexHtml = indexHtml.replace(new RegExp("\\[VERSION]", "g"), VERSION+new Date().getTime() );
  fs.writeFileSync(BUILD_OUTPUT+'/index.html', indexHtml);
});

gulp.task('copy:assets', function(){
  // copy assets
  gulp.src(['src/assets/**'])
    .pipe(gulp.dest(BUILD_OUTPUT_ASSETS));
});

gulp.task('dev', ['default'], function(){
  var devServer = require('./devServer');
  devServer();
});

gulp.task('test', shell.task(['npm run-script test']))

gulp.task('webpack:build',  function(callback) {
  var myConfig = Object.create(webpackConfig);
  webpack(myConfig, function(err, stats) {
    if (err) {
      throw new gutil.PluginError('webpack:build', err);
    }

    if(stats.compilation.errors.length > 0){
        gutil.log('[webpack:build]', stats.compilation.errors.toString({ colors: true }));
        process.exit(1);
    }

    callback();
  });
});
