#!/usr/bin/env groovy


node {
  def commmit_id
  def email = 'eng@dxchain.com'
  def has_error = false
  def error_msg = ''

  def root11 = tool name: 'Go 1.11', type: 'go'
  try {
    stage('Pre-Build Go11') {

      withEnv(["GOPATH=${WORKSPACE}", "PATH+GO=${root11}/bin:${WORKSPACE}/bin", "GOBIN=${WORKSPACE}/bin"]){
        // pipeline
        checkout scm
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && go version"
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && git rev-parse --short HEAD > .git/commit-id"
        commit_id = readFile("/var/jenkins_home/workspace/$JOB_NAME/src/github.com/DxChainNetwork/godx/.git/commit-id").trim()
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && go get -u -v github.com/kardianos/govendor"
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && govendor sync -v"
      }
    }
  } catch (err) {
    has_error = true
    error_msg = error_msg + "${err};"
  }

  try {
    stage ('Build Go11') {
      withEnv(["GOPATH=${WORKSPACE}", "PATH+GO=${root11}/bin:${WORKSPACE}/bin", "GOBIN=${WORKSPACE}/bin"]){
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && make gdx"
      }
    }
  } catch (err) {
    has_error = true
    error_msg = error_msg + "${err};"
  }

  try {
    stage ('Unit Test Go11') {
     withEnv(["GOPATH=${WORKSPACE}", "PATH+GO=${root11}/bin:${WORKSPACE}/bin", "GOBIN=${WORKSPACE}/bin"]){
        // prepare test package
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && make test"
      }
    }
  } catch (err) {
    has_error = true
    error_msg = error_msg + "${err};"
  }

  // def root10 = tool name: 'Go 1.10', type: 'go'
  // try {
  //   stage('Pre-Build Go10') {

  //     withEnv(["GOPATH=${WORKSPACE}", "PATH+GO=${root10}/bin:${WORKSPACE}/bin", "GOBIN=${WORKSPACE}/bin"]){
  //       // pipeline
  //       checkout scm
  //       sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && go version"
  //       sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && git rev-parse --short HEAD > .git/commit-id"
  //       commit_id = readFile("/var/jenkins_home/workspace/$JOB_NAME/src/github.com/DxChainNetwork/godx/.git/commit-id").trim()
  //       sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && go get -u -v github.com/kardianos/govendor"
  //       sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && govendor sync -v"
  //     }
  //   }
  // } catch (err) {
  //   has_error = true
  //   error_msg = error_msg + "${err};"
  // }

  // try {
  //   stage ('Build Go10') {
  //     withEnv(["GOPATH=${WORKSPACE}", "PATH+GO=${root10}/bin:${WORKSPACE}/bin", "GOBIN=${WORKSPACE}/bin"]){
  //       sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && make gdx"
  //     }
  //   }
  // } catch (err) {
  //   has_error = true
  //   error_msg = error_msg + "${err};"
  // }

  // try {
  //   stage ('Unit Test Go10') {
  //    withEnv(["GOPATH=${WORKSPACE}", "PATH+GO=${root10}/bin:${WORKSPACE}/bin", "GOBIN=${WORKSPACE}/bin"]){
  //       // prepare test package
  //       sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && make test"
  //     }
  //   }
  // } catch (err) {
  //   has_error = true
  //   error_msg = error_msg + "${err};"
  // }

  def root12 = tool name: 'Go 1.12', type: 'go'
  try {
    stage('Pre-Build Go12') {

      withEnv(["GOPATH=${WORKSPACE}", "PATH+GO=${root12}/bin:${WORKSPACE}/bin", "GOBIN=${WORKSPACE}/bin"]){
        // pipeline
        checkout scm
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && go version"
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && git rev-parse --short HEAD > .git/commit-id"
        commit_id = readFile("/var/jenkins_home/workspace/$JOB_NAME/src/github.com/DxChainNetwork/godx/.git/commit-id").trim()
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && go get -u -v github.com/kardianos/govendor"
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && govendor sync -v"
      }
    }
  } catch (err) {
    has_error = true
    error_msg = error_msg + "${err};"
  }

  try {
    stage ('Build Go12') {
      withEnv(["GOPATH=${WORKSPACE}", "PATH+GO=${root12}/bin:${WORKSPACE}/bin", "GOBIN=${WORKSPACE}/bin"]){
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && make gdx"
      }
    }
  } catch (err) {
    has_error = true
    error_msg = error_msg + "${err};"
  }

  try {
    stage ('Unit Test Go12') {
     withEnv(["GOPATH=${WORKSPACE}", "PATH+GO=${root12}/bin:${WORKSPACE}/bin", "GOBIN=${WORKSPACE}/bin"]){
        // prepare test package
        sh "cd $GOPATH/src/github.com/DxChainNetwork/godx && make test"
      }
    }
  } catch (err) {
    has_error = true
    error_msg = error_msg + "${err};"
  }

  if (has_error) {
    emailext attachLog: true, body: "Error message: ${error_msg}. More detail on console output at $BUILD_URL.", subject: 'Project: $JOB_NAME - Build # $BUILD_NUMBER - Status: failed', to: email
  }

}
