# UniFi Captive Portal

A [UniFi](https://www.ubnt.com) external captive portal which captures email
addresses and saves them to a [DynamoDB](https://aws.amazon.com/dynamodb/) table.

## Running

There are two important directories that need to be on disk in order for this
program to run: `assets` and `templates`. Assets holds CSS/JS/IMG assets.
Templates holds the various HTML templates.

If you would like to add custom elements (such as a header image) feel free.
The CSS library used is [Semantic UI](https://semantic-ui.com) so refer to their
documentation if you would like to modify the look.

You must have the DynamoDB table already created and have setup a shared
credentials file per the [SDK docs](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials).
Please use an IAM role with the least privileges required (read/write access to
the DynamoDB table you created). **DO NOT USE YOUR ROOT ACCOUNT**.

See the configuration section below for more information regarding the config
file. You will need to specify its location along with the assets and
templates directories.

Also, due to a limitation in the UniFi controller software, external portals
must run on port 80. I recommend running a reverse proxy (such as NGINX) in front
of this application instead of running it with elevated privileges.

```
unifi-captive-portal:
  -asset.dir string
    	Directory which contains css/js/img assets. (default "assets")
  -config.file string
    	Unifi captive portal configuration file. (default "unifi-portal.yml")
  -template.dir string
    	Directory which contains HTML templates. (default "templates")
  -verbose
    	Enable verbose/debug logging.
  -version
    	Print version/build information.
  -web.listen-address string
    	Address to listen on for requests. (default ":4646")
```

## Configuration

Config Key | Value
---------- | -----
unifi_url | Full URL of your UniFi Controller. Be sure to include the port it is running on (8443 is the default)
unifi_username | Username of the user to make API calls with. It is recommended to use a dedicated user
unifi_password | Password for user defined above
unifi_site | The name of the site the APs/Users reside in. Usually this is default
title | Title used in HTML pages as well as headings. Usually you will put your company name here
intro | Paragraph of text below the page title and above the form requesting a user for their email. You may wish to offer a brief explanation of why you are collecting their email address.
tos | Terms of Service. I am not a lawyer, the sample TOS provided is in no way legally binding nor implied valid. Please consult legal advice for what to put here.
minutes | Amount of time to register user for
redirect_url | URL to redirect users to if they do not provide one to the controller
dynamo_table_name | Name of your DynamoDB table to store the collected email addresses


## Building

Pre built binaries are provided on Github, but if you prefer to manually compile,
there is a Make file provided. The version variable is not required but highly
recommended.

    $ VERSION="version" make
