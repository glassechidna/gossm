# gossm

[![Build Status](https://travis-ci.org/glassechidna/gossm.svg?branch=master)](https://travis-ci.org/glassechidna/gossm)

`gossm` is a cross-platform command-line tool that makes it ridiculously simple 
to run shell commands remotely on your AWS EC2 instances. It is powered by the
[AWS Systems Manager service][sysman], which allows remote control through the
[SSM Agent][agent]. The agent is preinstalled on Windows, Amazon Linux and Ubuntu
AMIs. It can also be manually installed on others.

[sysman]: https://docs.aws.amazon.com/systems-manager/index.html
[agent]: https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent.html

## Installation

Download the latest release from [GitHub Releases][download]. There is a different
file for Mac, Windows and Linux. Choose the download for the computer *you* are
using, not the OS the EC2 instance is running.

[download]: https://github.com/glassechidna/gossm/releases

## Usage

Usage is as simple as specifying the instance(s) you want to run a command on
and the command you want to run. Specific instance(s) can be listed using `-i`
and multiple instances can be specified, e.g. `-i i-abc123 -i i-def456`. 
Alternatively, you can specify a tag using `-t` and all matching instances
will run the command, e.g. `-t tagname=value`. 

![Example screencap](https://user-images.githubusercontent.com/369053/47539940-82b5d100-d91e-11e8-88f8-42350e83eaa8.gif)

If you don't want interleaved output when running a command on multiple instances,
it can also be helpful to save the output to files. This can be done using `-f`.
This will save the files to `<cmd>/<instance id>.txt` in the current directory.

Additionally, you can specify an AWS profile using `--profile <name>` and
"quiet" mode using `-q`, which shows only command output and no metadata.
