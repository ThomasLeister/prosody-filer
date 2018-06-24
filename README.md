# Prosody Filer

A simple file server for handling XMPP http_upload requests. This server is meat to be used with the Prosody [mod_http_upload_external](https://modules.prosody.im/mod_http_upload_external.html) module.

**Why should I use this server?**

* Prosody's integrated http_upload server seems to be memory leaking.
* This server works without any script interpreters or additional dependencies. It is delivered as a binary.
* Go is very good at serving HTTP requests.

**Why shoud I NOT use this server?**

* This software is still work in progress


## Download 

If you are using regular x86_64 Linux, you can download a finished binary for your system on the [release page](https://github.com/ThomasLeister/prosody-filer/releases). **No need to compile this application yourself**. 


## Build (optional)

If you're using something different than a x64 Linux, you need to compile this application yourself.

To compile the server, you need a full Golang development environment. This can be set up quickly: https://golang.org/doc/install#install

Then checkout this repo: 

    go get github.com/ThomasLeister/prosody-filer

and switch to the new directory: 

    cd $GOPATH/src/github.com/ThomasLeister/prosody-filer

The application can now be build: 

    ### Build static binary
    ./build.sh

    ### OR regular Go build
    go build main.go


## Set up / configuration

[TBD]

### Setup Prosody Filter environment

Create a new user for Prosody Filer to run as: 

    adduser --disabled-login --disabled-password prosody-filer

Copy  

* the binary ```prosody-fiter``` and 
* config ```config.example.toml``` 

to ```/home/prosody-filer/```. Rename the configuration to ```config.toml```.


### Configure Prosody Filer

Prosody Filer configuration is done via the config.toml file in TOML syntax. There's not much to be configured:

* listenport: TCP port on which Prosody Filter should listen on. 
* secret: A random secret that matches the secret of http_upload_external in your Prosody configuration
* uploadRootDir: The path where upload should be stored.


### Systemd service file

Create a new Systemd service file: ```/etc/systemd/system/prosody-filer.service```

    [Unit]
    Description=Prosody file upload server

    [Service]
    Type=simple
    ExecStart=/home/prosody-filer/prosody-filer
    Restart=always
    WorkingDirectory=/home/prosody-filer
    User=prosody-filer
    Group=prosody-filer

    [Install]
    WantedBy=multi-user.target

Reload the service definitions, enable the service and start it: 

    systemctl daemon-reload
    systemctl enable prosody-filer
    systemctl start prosody-filer

Done! Prosody Filer is now listening on the specified port and waiting for requests.


### Configure Prosody

[make sure http_upload is disabled!]

Configure prosody.cfg.lua


### Configure Nginx

Create a new config file ```/etc/nginx/sites-available/upload.myserver.tld```:

    server {
        listen 80;
        listen [::]:80;
        listen 443 ssl;
        listen [::]:443 ssl;

        server_name uploads.myserver.tld;

        ssl_certificate /etc/letsencrypt/live/uploads.myserver.tld/fullchain.pem;
        ssl_certificate_key /etc/letsencrypt/live/uploads.myserver.tld/privkey.pem;

        client_max_body_size 50m;

        location /upload/ {
                proxy_pass http://127.0.0.1:5050/upload/;
        }
    }

Enable the new config:  

    ln -s /etc/ngin/sites-available/upload.myserver.tld /etc/nginx/sites-enabled/

Check Nginx config:

    nginx -t

Reload Nginx:

    systemctl reload nginx



## Automatic purge

Prosody Filer has no immediate knowlegde over all the stored files and the time they were uploaded, since no database exists for that. Also Prosody is not capable to do auto deletion if *mod_http_upload_external* is used. Therefore the suggested way of purging the uploads directory is to execute a purge command via a cron job:

    @daily       find /var/lib/prosody/uploads -maxdepth 0 -type d -mtime +28 | xargs rm -rf

This will delete uploads older than 28 days.  


