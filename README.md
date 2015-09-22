# Rino - Remote IRSSI Notifier - OSX version
IRSSI SSH notifier for MacOSX. It uses terminal-notifier to talk with
the notification center, SSH to publish its port on the remote host
and an IRSSI plugin to push notification to the listener.

## Under the hood
This is a small Golang/Perl/Bash application that allows you to send
notification on private messages and mentions from your remote IRSSI
to your local MacOSX notification center.

To achieve such awesomeness rino spawns a golang server (net.Listener)
on a local port. This port need to be forwarded to your remote IRSSI
box, where a perl plugins will send notification for privmsg / mentions
to the forwarded port. This will trigger a run of [terminal-notifier](https://github.com/julienXX/terminal-notifier)
and a nice notification with the message will pop on you screen.

Rino has only one mandatory argument (--link) which specifies the server
will run on and a tag to identify the notification.
```shell
rino -l crisidev:4223
```
To avoid problems, this tag need to be "whitelisted" for the IRSSI perl
plugin. The whitelisting can be done creating an empty file, named as
the link into /$HOME/.irssi/rino inside your remote IRSSI box.

If your IRSSI is on localhost, you just have to avoid the port
forwarding.

## Installation and usage
### Standalone
#### Prerequisites
* MacOSX
* [IRSSI](http://irssi.org/)
* [terminal-notifier](https://github.com/julienXX/terminal-notifier)
* [golang](https://golang.org/)
* [OpenSSH](http://www.openssh.com/)
* Nohup

#### Install it
* On your local machine
```shell
$ go get github.com/crisidev/rino
$ go install github.com/crisidev/rino
```
* On your IRSSI box
```shell
$ git clone github.com/crisidev/rino
$ cp rino/irssi_plugins/rino.pl /$HOME/.irssi/plugins/autorun
$ (from IRSSI) /script load autorun/rino.pl
```

#### Use it
ssh -R 4223:localhost:4223 $USER@irssibox
nohup rino -l irssi:4223 >> /$HOME/.rino/irssi:4223.log &

### Complete
#### Prerequisites
There are also utility scripts inside folders bin and ssh to allow
integration with autossh, tmux and ssh_config.
Take a look to folders bin and ssh and adapt the scripts to your
necessities.
