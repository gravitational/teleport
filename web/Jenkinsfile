pipeline {
	agent any

	options {
		skipDefaultCheckout(true)
	}

	stages {
		stage('checkout') {
			steps {
				sh "[ -d 'packages/webapps.e' ] && git submodule deinit -f packages/webapps.e"
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
