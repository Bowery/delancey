from fabric.api import *
import requests

project = 'delancey'
repository = 'git@github.com:Bowery/' + project + '.git'
hosts = [
  'bowery@64.71.188.179',
  'ubuntu@ec2-54-205-254-102.compute-1.amazonaws.com'
]
env.key_filename = '/home/ubuntu/.ssh/id_aws'
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

def deploy():
  execute(restart, hosts=hosts)
