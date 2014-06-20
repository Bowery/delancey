from fabric.api import *
import requests

project = 'delancey'
repository = 'git@github.com:Bowery/' + project + '.git'
hosts = [
  'bowery@64.71.188.179',
  'ubuntu@ec2-54-205-254-102.compute-1.amazonaws.com'
]
env.key_filename = '/home/ubuntu/.ssh/id_aws'
# env.key_filename = '/Users/steve/.ssh/bowery.pem'
env.password = 'java$cript'

@parallel
def restart():
  local_username = run('echo $(whoami)')
  local_path = '/home/' + local_username
  go_path = local_path + '/gocode'
  project_path = go_path + '/src/github.com/Bowery/' + project

  sudo('mkdir -p ' + project_path)
  with cd(project_path):
    run('git pull')
    with cd('delancey'):
      sudo('GOPATH=' + go_path + ' go get -d')
      sudo('GOPATH=' + go_path + ' go build')
      # sudo('mv delancey satellite')
      # sudo('cp -f satellite ' + project_path + '/images/bowery/')
      # sudo('rm -rf /satellite/*')
      # sudo('cp -rf ' + project_path + '/images/bowery/* /satellite/')
      # sudo('chown ' + local_username + ':' +local_username + ' /satellite/satellite')


def deploy():
  execute(restart, hosts=hosts)
