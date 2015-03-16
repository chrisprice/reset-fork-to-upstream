module.exports = function(grunt) {

  require('load-grunt-tasks')(grunt);

  grunt.initConfig({
    babel: {
      options: {
        sourceMap: true
      },
      dist: {
        files: {
          'dist/app.js': 'src/app.js'
        }
      }
    },
    browserify: {
      options: {
        browserifyOptions: {
         debug: true
        }
      },
      dist: {
        files: {
          'dist/index.js': ['dist/app.js'],
        }
      }
    },
    less: {
      dist: {
        files: {
          'dist/index.css': 'src/app.less'
        }
      }
    },
    watch: {
      dist: {
        files: ['src/**/*.js', 'src/**/*.less', 'index.html'],
        tasks: ['scripts', 'styles'],
        options: {
          livereload: true
        },
      }
    },
  });

  grunt.registerTask('scripts', ['babel', 'browserify']);
  grunt.registerTask('styles', ['less']);
  grunt.registerTask('default', ['scripts', 'styles', 'watch']);

};
