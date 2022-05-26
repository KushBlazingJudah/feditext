# Moderation on Feditext

Feditext has a single `/admin` page that allows you to manage parts of your
instance.
But, you first need to log in with a username and password that was configured
using `feditext create`.
If you haven't done so already, you should reread the example configuration
file.

There's a few things you can do from here:

- add, update, or remove boards
- follow instances or (currently broken) unfollow instances
- fetch the posts of other instances
- post and delete news
- modify and update privileges for other moderators
- see reports

The UI isn't very fleshed out however works well enough to get the job done, it
may just not be very obvious.
You can update boards and moderators by simply putting their ID/username into
their respective textbox, and whatever new values that are provided will be
used.

**Warning:** Due to how authentication is done and to prevent querying the
database, whatever privileges you log in with will be permanent until the token
you got when you logged in expires.
So, if you change someone's privileges to 0, they won't know until they obtain a
new token, by deleting the old one or waiting for it to expire.

Outside of the `/admin` page, you can also:

- delete posts
- force a post to be sent again to other instances
- ban a user if the instance is not in private mode
- post without a captcha
