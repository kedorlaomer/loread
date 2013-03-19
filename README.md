loread
======

An NNTP client implemented in Google's go. The client should download articles,
thread them (see threading.txt) and present them as a local HTTP server, such
that they can be read without network connectivity.

Downloading, parsing and saving articles work. Rough threading (steps 1-3 from
threading.txt) seem to work, but are not tested thoroughly.

*TODO*:
+ Threading
+ HTML based user interface
