loread
======

An NNTP client implemented in Google's go. The client should download articles,
thread them (see threading.txt) and present them as a local HTTP server, such
that they can be read without network connectivity.

Downloading, parsing and saving articles works. Threading seems to work, but is
not tested thoroughly.

*TODO*:
+ HTML based user interface
