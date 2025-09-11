# boylstonchessclub-tdbot
Tournament & player information bot for the Boylston Chess Club
(https://boylstonchess.org/)

# Usage 

Available Commands:
```
  /td help               This help screen

  /td about              Show information regarding this Boylston
                         Chess Club TD Bot

  /td cal [days: <days>] [broadcast: <true|false>]
                         Show upcoming events over the specified
                         number of days (14 by default if not
                         specified). To share with the channel set
                         broadcast: true (false by default).

  /td entries eventid: <eventId> [broadcast: <true|false>]
                         Display current entries for a tournament,
                         grouped by section. To share with the channel set
                         broadcast: true (false by default).

  /td event eventid: <eventId> [broadcast: <true|false>]
                         Retrieve detailed information regarding an
                         event. To share with the channel set
                         broadcast: true (false by default).

  /td pairings eventid: <eventId> [broadcast: <true|false>]
                         Display current pairings for a tournament,
                         grouped by section. To share with the channel set
                         broadcast: true (false by default).

  /td player memid: <memberId> [broadcast: <true|false>]
                         Display information on a specific player
                         given their USCF member id. To share with the
                         channel set broadcast: true (false by
                         default).

  /td standings eventid: <eventId> [broadcast: <true|false>]
                         Display current standings for a tournament,
                         grouped by section. To share with the channel set
                         broadcast: true (false by default).

```

# Installing into your Discord Server

https://discord.com/oauth2/authorize?client_id=1381308091169243227&permissions=274877908992&integration_type=0&scope=bot+applications.commands
