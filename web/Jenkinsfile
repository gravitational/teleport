pipeline {
	agent any

	options {
		skipDefaultCheckout(true)
	}

	stages {
		stage('checkout') {
			steps {
				sh "if [ -d 'packages/webapps.e' ]; then git submodule deinit -f packages/webapps.e; fi"
				checkout scm
			}
		}
		stage('test') {
			steps {
				sh "make clean check"
			}
		}
		stage('gather artifacts') {
			steps {
				sh "make dist packages/webapps.e/dist"
			}
		}
	}

	post {
		success {
			archiveArtifacts artifacts: 'dist/**/*,packages/webapps.e/dist/**/*', fingerprint: true
		}
	}
}
