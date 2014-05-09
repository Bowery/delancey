# Crosswalk
A lightweight agent for running local files against your production environment.

## Intro
Welcome to the introduction to Crosswalk! This is the best place to start. We cover what Crosswalk is, how it compares to existing solutions, what problem it solves, and installation instructions. If you're familiar with Crosswalk, the [documentation]('') section provides in-depth explanations of everything Crosswalk can do.

## What is Crosswalk?
Crosswalk is a tool for developing web applications in their production environments. While developing, most software developers run their code in a development environment (Virtual Machine, Personal Computer, Web IDE, etc.) that differs from the production servers the code will eventually run on. The parity between these environments causes a variety of unexpected bugs that can be avoided by developing software in the same environment it will eventually run in. Adam Wiggins, founder of Heroku, has [written extensively about this Problem](http://12factor.net/dev-prod-parity).

While developers writing mission critical applications already develop on clones of their production environments, they take a significant productivity hit in doing so. Everytime they change code, they must deploy that code and restart the application. Crosswalk automates this process.

Keep your source code on your computer and edit it with your favorite text editor. Crosswalk will detect changes, update the files on the server running your environment, and restart the application. This allows for the optimal development process. The productivity of developing locally, with the quality of assurance of having run your code in the environment you're deploying it to.

## Architecture
An agent runs on the server you want to run your code on. It will bind to port 3000 and must be accessible by the command-line interface.

The command-line interface runs on the developers computer and will is passed information about where the source code is and how to run it.

## Crosswalk vs. Other Software
### RSync
A common approach to developing on a remote server is to upload code using RSync over ssh every time a file is changed. This is the most manual way possible to develop on a remote server. The workflow is as follows:
1. Setup your ssh keys to work with the server. An in depth tutorial can be found [here](http://askubuntu.com/questions/4830/easiest-way-to-copy-ssh-keys-to-another-machine/4833#4833)
2. Change Code
3. rsync -avz -e "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null" --progress code root@10.0.0.10:/root/code
4. ssh into the server
5. manually install dependencies and restart the necessary processes
6. See Changes!

Crosswalk's workflow is:
1. Change Code
2. See Changes!

Rsync is a great tool for syncing large files across servers, but it wasn't made with the developer workflow in mind.

### Dropbox
Dropbox is actually [built on RSync](https://github.com/dropbox/librsync), but automates a lot of the setup making a workflow that is similar to Crosswalk. The problem with using dropbox for development is the update latency (30-120 seconds) caused by Dropbox's Architecture:
Your Computer -> Dropbox's Server(s)/Backups/Routers -> Your Server

Additionally, Dropbox isn't made with the developer workflow in mind, so it won't restart your application and install dependencies for you. You'll have to set up ssh, shell into the server, manually install dependencies and restart the application every time you want to see your changes.

Crosswalk sends files directly to the server and depending on where your server is located can update and restart your application in under 200ms. That's 300x as fast as dropbox.

### Web IDEs
There are a number of web services that allow you to edit and run your code online via a web page. The primary problem with these services is that they don't let you use the editor that you want. You have to use the web editor which is often slower than desktop editors.

Some of these services are started to let you use whatever editor you want, but they aren't actually running your code in the environment you will use in production. Web IDEs are great for interview collaboration and people learning how to code, but they lack the flexibility for mission critical software development.

Crosswalk is committed to its flexible architecture that can support every operating system, text editor, stack, library, cloud provider, and anything else you might have in your production environment.

### Vagrant
In 2010, Vagrant seriously changed the development environment game. Today, it's used by Apple, Microsoft, Mozilla, Nokia, & Others. Vagrant allows you to automate the provisioning of local virtual machines by writing a Vagrantfile. It's a powerful, but complex system for configuring virtual machines.

To begin with, it takes a lot of work to write a Vagrantfile and is sometimes impossible to model advanced network, storage, & virtualization solutions used in production. Even if you're able to write a perfect Vagrantfile, sooner or later, the development environment provisioned by Vagrant gets out of sync with the production environment.

Crosswalk runs on your production stack which you have to spend time setting up out of necessity, so there's not extra time configuring your environment. Crosswalk also runs on your production environment, so there's never any concern about not being able to support something you're using in production.

## Installation
### Agent
[Download](http://crosswalk.io/download) the agent on your server.
```
$ agent --password SUP3RS3CUR3 --dir /desired/app/path
```

### CLI
[Download](http://crosswalk.io/download) the agent on your computer.
```
$ crosswalk help
Usage: crosswalk [options]

Options:
  --addr         Address of Agent.
  --start        Command to start application.
  --test         Command to test application.
  --build        Command to build application. It's always run before the Start Command.
  --path         Path to sync files from.

$ crosswalk --addr 10.0.0.10
            --start ./app
            --build make
            --test "make test"
            --path /src
            --password SUP3RS3CUR3
```

## Tutorials
- [Digital Ocean]('')
- [AWS]('')
- [Google Compute Engine]('')
- [Vagrant (offline-mode)]('')
