var gulp = require("gulp");
var webpack = require('webpack');
var webpackConfig = require('./webpack.config.js');
var gutil = require('gulp-util');
var shell = require('gulp-shell');
var fs = require('fs');

var VERSION = '0.1';

gulp.task('default', ['copy:assets', 'copy:html', 'webpack:build']);

gulp.task('copy:html', function(){
  var indexHtml  = fs.readFileSync('src/index.html', 'utf8');
  indexHtml = indexHtml.replace(new RegExp("\\[VERSION]", "g"), VERSION+new Date().getTime() );
  fs.writeFileSync('dist/index.html', indexHtml);
});

gulp.task('copy:assets', function(){
  // copy mocks
  gulp.src(['src/mocks/**'])
    .pipe(gulp.dest('dist/mocks'));

  // copy assets
  gulp.src(['src/assets/**', 'src/mocks/**'])
    .pipe(gulp.dest('dist/assets'));
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
