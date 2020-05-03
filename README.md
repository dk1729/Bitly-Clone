# Bitly-Clone
The bitly clone consists of 3 microservices
1. Control Panel
2. Link Redirection 
3. Trend Server

As requests to make a url come to the control panel, they are forwarded to rabbit-mq. Rabbit-mq forwards the request to sql servers for storing. 

Now, when we hit a shortened link, we are taken to the redirection server. A count increments in order to keep a track of trends. If the count>5, we save that in the trend server. We check the trend server if there is a cached value in it. If not, we get it from mysql server.


Youtube video link : https://youtu.be/uRI75sf9jvc
