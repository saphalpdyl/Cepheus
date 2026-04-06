## Setup development environment

I like to work on the host during networking projects, hence my workflow includes `rsync`ing to the VM, rebuild and redeploy. Since doing rsync everytime is tedious, I use `emeraldwalk.runonsave` extension ( included in `.vscode/extensions.json` as a recommendation), that runs the rsync command when files changes are saved.

To setup, after the VM is up, run `vagrant ssh-config` and append it to your ssh config ( usually `~/.ssh/config`). **Be sure to change the Host from default to cepheus.**

After that, this configuration to `.vscode/settings.json` will complete the rsync on save workflow.

```json
    "emeraldwalk.runonsave": {
        "commands": [
            {
                "match": ".*",
                "cmd": "rsync -rav . cepheus:/home/vagrant/cepheus/ --exclude-from=.rsyncignore"
            }
        ]
    },
```