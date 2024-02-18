# Prosody Filer

A simple file server for handling XMPP http_upload requests. This server is meant to be used with the Prosody [mod_http_upload_external](https://modules.prosody.im/mod_http_upload_external.html) module.

**Despite the name, this server is also compatible with Ejabberd and Ejabberd's http_upload module!**

---

## Why should I use this server?

Originally this software was written to circumvent memory limitations / issues with the Prosody-internal http_upload implementation at the time. These limitations do not exist, anymore. Still this software can be used with Ejabberd and Prosody as an alternative to the internal http_upload servers. 


## Download

If you are using regular x86_64 Linux, you can download a finished binary for your system on the [release page](https://github.com/ThomasLeister/prosody-filer/releases). **No need to compile this application yourself**.


## Build (optional)

If you're using something different than a x64 Linux, you need to compile this application yourself.

To compile the server, you need a full Golang development environment. This can be set up quickly: https://golang.org/doc/install#install

Then checkout this repo:

```sh
go install github.com/ThomasLeister/prosody-filer
```

and switch to the new directory:

```sh
cd $GOPATH/src/github.com/ThomasLeister/prosody-filer
```

The application can now be build:

```sh
### Build static binary
./build.sh

### OR regular Go build
go build prosody-filer.go
```


## Set up / configuration


### Setup Prosody Filer environment

Create a new user for Prosody Filer to run as:

```sh
adduser --disabled-login --disabled-password prosody-filer
```

Switch to the new user:

```sh
su - prosody-filer
```

Copy

* the binary ```prosody-filer``` and
* config ```config.example.toml```

to ```/home/prosody-filer/```. Rename the configuration to ```config.toml```.

Make sure the `prosody-filer` binary is executable: 

```
chmod u+x prosody-filer
```


### Configure Prosody

Back in your root shell make sure ```mod_http_upload``` is **dis**abled and ```mod_http_upload_external``` is **en**abled! Then configure the external upload module:

```lua
Component "uploads.myserver.tld" "http_upload_external"
    http_upload_external_base_url = "https://uploads.myserver.tld/upload/"
    http_upload_external_secret = "mysecret"
    http_upload_external_file_size_limit = 50000000 -- 50 MB
```

Restart Prosody when you are finished:

    systemctl restart prosody


### Alternative: Configure Ejabberd

Although this tool is named after Prosody, it can be used with Ejabberd, too! Make sure you have a Ejabberd configuration similar to this:

```yaml
  mod_http_upload:
    put_url: "https://uploads.@HOST@/upload"
    external_secret: "mysecret"
    max_size: 52428800
```


### Configure Prosody Filer

Prosody Filer configuration is done via the `config.toml` file in TOML syntax. There's not much to be configured. The most important piece is the `secret` setting, which **needs to match the secret defined in your mod_http_upload_external settings!**

```toml
### IP address and port to listen to, e.g. "[::]:5050"
listenport      = "[::1]:5050"

### Secret (must match the one in prosody.conf.lua!)
secret          = "mysecret"

### Where to store the uploaded files
storeDir        = "./upload/"

### Subdirectory for HTTP upload / download requests (usually "upload/")
uploadSubDir    = "upload/"
```


In addition to that, make sure that the nginx user or group can read the files uploaded
via prosody-filer if you want to have them served by nginx directly.


### Docker usage 

To build container:

```docker build . -t prosody-filer:latest```

To run container use:

```docker run -it --rm -v $PWD/config.example.toml:/config.toml prosody-filer -config /config.toml```


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
    # Group=nginx  # if the files should get served by nginx directly:

    [Install]
    WantedBy=multi-user.target

Reload the service definitions, enable the service and start it:

    systemctl daemon-reload
    systemctl enable prosody-filer
    systemctl start prosody-filer

Done! Prosody Filer is now listening on the specified port and waiting for requests.

### FreeBSD service file

- First, install go and build the static binary with `go install github.com/ThomasLeister/prosody-filer`.
The binary should be in `~/go/bin/prosody-filer`. 
- Create an user with `pw user add -n prosodyfiler -c 'Prosody Filer' -d /home/prosodyfiler -m -s /usr/sbin/nologin` 
and copy the binary into `/home/prosodyfiler/`.
- Put this rc script in `/etc/rc.d/prosody-filer`:
```
#!/bin/sh

# PROVIDE: prosody_filer
# REQUIRE: FILESYSTEMS networking
# KEYWORDS: upload

. /etc/rc.subr

name="prosody_filer"
program_name="prosody-filer"
title="Prosody-Filer"
rcvar=prosody_filer_enable
prosody_filer_user="prosodyfiler"
prosody_filer_group="prosodyfiler"
prosody_filer_chdir="/home/prosodyfiler/"

pidfile="/home/prosodyfiler/${program_name}.pid"
required_files="/home/prosodyfiler/config.toml"
exec_path="/home/prosodyfiler/${program_name}"
output_file="/var/log/${program_name}.log"

command="/usr/sbin/daemon"
command_args="-r -t ${title} -o ${output_file} -P ${pidfile} -f ${exec_path}"

load_rc_config $name
```
- Execute `sysrc prosody_filer_enable=YES`

You can now start the service via `service prosody_filer start` and check
it's log via `/var/log/prosody-filer.log`.

### Configure Nginx

Create a new config file ```/etc/nginx/sites-available/uploads.myserver.tld```:

```nginx
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
        if ( $request_method = OPTIONS ) {
                add_header Access-Control-Allow-Origin '*';
                add_header Access-Control-Allow-Methods 'PUT, GET, OPTIONS, HEAD';
                add_header Access-Control-Allow-Headers 'Authorization, Content-Type';
                add_header Access-Control-Allow-Credentials 'true';
                add_header Content-Length 0;
                add_header Content-Type text/plain;
                return 200;
        }

        proxy_pass http://[::]:5050/upload/;
        proxy_request_buffering off;
    }
}
```

Enable the new config:

    ln -s /etc/nginx/sites-available/uploads.myserver.tld /etc/nginx/sites-enabled/

Check Nginx config:

    nginx -t

Reload Nginx:

    systemctl reload nginx


#### Alternative configuration for letting Nginx serve the uploaded files

*(not officially supported - user contribution!)*

```nginx
server {
    listen 80;
    listen [::]:80;
    listen 443 ssl;
    listen [::]:443 ssl;

    server_name uploads.myserver.tld;

    ssl_certificate /etc/letsencrypt/live/uploads.myserver.tld/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/uploads.myserver.tld/privkey.pem;

    location /upload/ {
        if ( $request_method = OPTIONS ) {
                add_header Access-Control-Allow-Origin '*';
                add_header Access-Control-Allow-Methods 'PUT, GET, OPTIONS, HEAD';
                add_header Access-Control-Allow-Headers 'Authorization, Content-Type';
                add_header Access-Control-Allow-Credentials 'true';
                add_header Content-Length 0;
                add_header Content-Type text/plain;
                return 200;
        }

        root /home/prosody-filer;
        autoindex off;
        client_max_body_size 51m;
        client_body_buffer_size 51m;
        try_files $uri $uri/ @prosodyfiler;
    }
    location @prosodyfiler {
        proxy_pass http://[::1]:5050;
        proxy_buffering off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host:$server_port;
        proxy_set_header X-Forwarded-Server $host;
        proxy_set_header X-Forwarded-For $remote_addr;
    }
}
```

## apache2 configuration (alternative to Nginx)

*(This configuration was provided by a user and has never been tested by the author of Prosody Filer. It might be outdated and might not work anymore)*

```
<VirtualHost *:80>
    ServerName upload.example.eu
    RedirectPermanent / https://upload.example.eu/
</VirtualHost>

<VirtualHost *:443>
    ServerName upload.example.eu
    SSLEngine on

    SSLCertificateFile "Path to the ca file"
    SSLCertificateKeyFile "Path to the key file"

    Header always set Public-Key-Pins: ''
    Header always set Strict-Transport-Security "max-age=63072000; includeSubdomains; preload"
    H2Direct on

    <Location /upload>
        Header always set Access-Control-Allow-Origin "*"
        Header always set Access-Control-Allow-Headers "Content-Type"
        Header always set Access-Control-Allow-Methods "OPTIONS, PUT, GET"

        RewriteEngine On

        RewriteCond %{REQUEST_METHOD} OPTIONS
        RewriteRule ^(.*)$ $1 [R=200,L]
    </Location>

    SSLProxyEngine on

    ProxyPreserveHost On
    ProxyRequests Off
    ProxyPass / http://localhost:5050/
    ProxyPassReverse / http://localhost:5050/
</VirtualHost>
```


## Automatic purge

Prosody Filer has no immediate knowlegde over all the stored files and the time they were uploaded, since no database exists for that. Also Prosody is not capable to do auto deletion if *mod_http_upload_external* is used. Therefore the suggested way of purging the uploads directory is to execute a purge command via a cron job:

    @daily    find /home/prosody-filer/upload/ -mindepth 1 -type d -mtime +28 -print0 | xargs -0 -- rm -rf

This will delete uploads older than 28 days.


## Check if it works

Get the log via

    journalctl -f -u prosody-filer

If your XMPP clients uploads or downloads any file, there should be some log messages on the screen.
