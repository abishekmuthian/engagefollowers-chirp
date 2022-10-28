## engage followers
Engage with your Twitter followers, Build meaningful relationships - [https://engagefollowers.com](https://engagefollowers.com).

### What
engage followers classifies the tweets of **our followers** according to our interests and helps us engage with their tweets to build meaningful relationships with our Twitter followers.

### Why
Be it 10 followers or 10 million followers, Someone follows us on Twitter because there's a common interest; But what is that?

Knowing the common interests with our followers can help build meaningful relationships with them.

### How
Here's where **engage followers** does its magic! All we need to do is connect our Twitter account and set our favorite topics.

Tweets from our followers are classified using Machine Learning (So it's not just based on keywords). We can choose to receive an email digest at the end of the day containing those tweets which matches our topics (and/or) automatically like the tweet to find them in the Likes section sooner.

We can then choose to engage with those tweets from our followers to build meaningful relationships with them.

### Demo
[![engage followers demo video](backup/demo/video_thumbnail.png)](https://www.youtube.com/watch?v=9ANVQBrb9OE)
Clicking the above image would open the demo video in YouTube.

### Technical How
1. A 100 randomized Twitter followers are stored in a Twitter list (The list is rotated every 24 hours). 
2. Tweets from the members of the list are retrieved at regular intervals.
3. New tweets are classified using a BART model using our set **topics of interest** as labels.
4. Tweets which matches our interests are stored to be sent via email digest and/or liked.
5. An email digest containing the tweets from the followers, Classified according to our interests are sent.

## Testing Setup

### Requirements
1. Go version 1.18.5 (or)  higher.
2. Python version 3.6.9 (or) higher.
3. PostgreSQL version 12 (or) higher.
4. Redis server version 7.0.4 (or) higher.

### Database
1. Create a PostgreSQL database (e.g. **engagefollowers_development**) and create tables as per the .sql file located in `/backup/db/Create-Tables.sql`.

Note: If a new role(user) is created for testing other than default `postgres` user, The user needs to be provided proper permissions to the database and tables(Currently commented in the .sql file).

2. Make the redis server [persist data](https://redis.io/docs/manual/persistence/) by enabling `Append-only file`(AOF).

### Machine Learning Server
Install `uvicorn`:
        
        $ pip3 install "uvicorn[standard]"

Install the required Python packages from `requirements.txt`.

1. Create a folder `models` in the same directory where `engagefollowers` project folder is located.

        $ mkdir models
   
2. Download the hugging face transformer model to the models directory.

        $ cd models
        $ git lfs install
        $ git clone https://huggingface.co/facebook/bart-large-mnli

3. Run the service in the engagefollowers folder, 

        $ cd engagefollowers
        $ uvicorn main:app --host 0.0.0.0 --port 8000

Note: 
1. In production, I use gunicorn with multiple workers for greater performance as found in `/backup/server_scripts/gunicorn.service`. For using [gunicorn](https://gunicorn.org/), It needs to be installed along with uvicorn as detailed earlier.

2. GPU can be used in the Machine Learning service if `device=0` used while initializing model in the `main.py`.

### Twitter Developer Setup
1. Get access to developer portal at Twitter - [https://developer.twitter.com/](https://developer.twitter.com/).
2. Create a new project `engagefollowers-test` and a new app `engagefollowers-test`.
3. Type of App: `Web App`.
4. Get the `Client ID`, `Client Secret`.
5. Setup OAuth 2.0,
   
   a. Callback URI / Redirect URL:  `[https project domain]/users/connect`.

   b. Website URL: `[https project domain]`

Note: For local testing, A reverse proxy service with ssl like ngrok can be used. 
After launching the application,
        
        ./ngrok http 3000


### Configuration
The configuration file is located in `/secrets/fragmenta.json`.

Use the `development{}` object for testing.

The following fields are mandatory for the function of the application -

1. `db_url`: URL for the postgres database server. Replace `[password]` and `[IP address]` with your postgres credential and server ip address.

Note: `db`, `db_pass`, `db_user`, `db_port`, can also be optionally used instead of `db_url` and if the db user is other than `postgres`, Proper permissions for the database needs to be provided as per `/backup/db/Create-Tables.sql`.

2. `port`: Port for the application, `3000` is set by default in the configuration.
3. `root_url`: **[https project domain]** mentioned earlier in the Twitter developer portal setup.
4. `client_Id`: **Client ID** mentioned earlier in the Twitter developer portal setup.
5. `client_secret`: **Client Secret** mentioned earlier in the Twitter developer portal setup.
6. `classifier_server`: **Machine Learning Server** created earlier, `[Machine Learning Server IP]:8000/classification`.
7. `redis_server`: `[Redis server IP]:6379`.
8. `twitter_redirect_uri`: `[https project domain]/users/connect`.

### Project Structure
1. `server.go` is the entry point.
2. `/src/users/actions` contains the business logic.

### Scheduling the services

`/src/app/services.go` file consists of timings for scheduling the Tweet retrieval and  Email Digest service.

1. `updateInterval` and `updateTime` is used for Twitter follower and Tweets retrieval which are set to `15 minutes` and `2 seconds` respectively for testing. 
2. `emailInterval` and `emailTime` is used for sending email digest and is set to `24 hours` and `60 seconds` respectively for testing.

### Building the application
After ensuring Go is installed in the system,
        
        $ cd engagefollowers-chirp
        $ go build

### Run the application
After ensuring that Postgres, Redis and Machine Learning server is running,

    $ ./engagefollowers

### Usage
1. Visit the engage followers project domain you have set in `root_url` with set `port` e.g. `localhost.com:3000`.
2. Register a user account (With a valid email address).
3. Connect your twitter account (With couple of followers with recent tweets) and Authorize the app.
4. Set the topics for classification.
5. **Email Digest** is required (selected by default) for receiving the classified tweets by email and/or **Auto Like** for automatically liking those tweets to find them in the Likes section of your Twitter profile sooner.

### Miscellaneous Testing Notes
1. Tweet ids are stored in redis after processing to avoid redundant processing, So for testing if there are no new tweets from the followers, the application needs to be run when there are new tweets(Or should tweet from a follower account before each test).
2. Twitter rate limits are handled internally (e.g. Follower lists are created only every 24 hours), Although **rate limits and other Twitter warnings/errors would be printed in the log, It doesn't affect the state of the application**.

### Copyright and Licenses
© 2022 Abishek Muthian https://engagefollowers.com.

Private repository submitted as an entrant for [chirp dev challenge 2022](https://chirpdevchallenge.devpost.com/).

### Open-Source Licenses
Licenses for open-source libraries used in this project.

Fragmenta: https://github.com/fragmenta licensed under [The MIT License](https://github.com/fragmenta/fragmenta-cms/blob/master/LICENSE).

gotwi: https://github.com/michimani/gotwi licensed under [The MIT License](https://github.com/michimani/gotwi/blob/main/LICENCE).

Go-OAuth1.0: https://github.com/klaidas/go-oauth1 licensed under [The MIT License](https://github.com/klaidas/go-oauth1/blob/master/LICENSE).

resty: https://github.com/go-resty/resty licensed under [The MIT License](https://github.com/go-resty/resty/blob/master/LICENSE).

schema: https://github.com/gorilla/schema licensed under [BSD 3-Clause "New" or "Revised" License](https://github.com/gorilla/schema/blob/master/LICENSE)

stripe-go: https://github.com/stripe/stripe-go licensed under [The MIT License](https://github.com/stripe/stripe-go/blob/master/LICENSE).

fastapi: https://github.com/tiangolo/fastapi licensed under [The MIT License](https://github.com/tiangolo/fastapi/blob/master/LICENSE).

transformers: https://github.com/huggingface/transformers licensed under [Apache License 2.0](https://github.com/huggingface/transformers/blob/main/LICENSE).