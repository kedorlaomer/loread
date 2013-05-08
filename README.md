loread
======

An NNTP client implemented in Google's go. The client should download articles,
thread them (see threading.txt) and present them as a local HTTP server, such
that they can be read without network connectivity.

Downloading, parsing and saving articles works. Threading seems to work, but is
not tested thoroughly.

Configuration
=============

The configuration resides in a file config.txt that might look like

server: reader80.eternal-september.org
port: 80
login: some-user-name
pass: top-secret
groups: comp.lang.lisp, rec.alt.coolstuff
fetch-maximum: 10000
verbose: yes

Comments or anything other than this formatting (including omitting or adding
spaces) is not allowed, although adding other keys is not a problem.

Key words
---------

 + _server_: URL
 + _port_: NNTP port, usually 119
 + _login_: login name
 + _pass_: password, sent when requested **without encryption**
 + _groups_: subscribed groups (comma-and-space separated)
 + _fetch-maximum_: for the initial loading, how many articles should we fetch?
 + _verbose_: should we print the transcript of client/server communication

The local server listens on port 8080 (this currently can't be changed).

**TODO**:
 + mark links
 + show emoticons (e. g. as animated (?) GIFs)
 + mark links as <a href="https://github.com/kedorlaomer/loread">links</a>
 + deal with double spacing in posts written by Google Groups
