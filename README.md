CyberInu Slack Profile updater
==============================

Generates a new image of CyberInu pictures (art by [agaberu](https://twitter.com/agaberu))
with the current time on his eyes and sets it as Slack profile picture.

Screenshots
-----------

![Model 0](screenshots/model0.png?raw=true)
![Model 1](screenshots/model1.png?raw=true)
![Model 2](screenshots/model2.png?raw=true)
![Model 3](screenshots/model3.png?raw=true)

Usage
-----

    Usage of cyberinu:
      -log-file string
            File to log to
      -model-index int
            Model index to render (default -1)
      -output string
            Generate the image and save it as a file instead of uploading to slack
      -request-timeout duration
            Request timeout (default 45s)
      -seconds-offset int
            Seconds after which we generate picture for the next minute (default 30)
      -slack-token string
            Slack token
      -update-interval duration
            Update interval (default 1m0s)

Deployment with Docker
----------------------

You can build a standalone docker image using:

    docker build -t cyberinu:latest .

You can then run the container on with:

    docker run -d --name cyberinu -e TZ=Europe/Paris -e SLACK_TOKEN=<slack_token> cyberinu:latest

The token must be a Slack Legacy Token, and can be created/requested per workspace
[here](https://api.slack.com/custom-integrations/legacy-tokens)

You may want to omit `-e TZ=Europe/Paris` when running on a machine with same timezone as yours,
or change it to fit your target timezone ([See list](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones))
