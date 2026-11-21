# 0003: A tab, window or group id is `<profile>:<chrome id>`, printed and read in full

Every profile that loads the extension has its own host, and Chrome, Chromium and Brave can run at
once. Chrome's numbers are unique in one browser process only; the profile part, a random id that
the extension makes once per profile, makes them unique across browsers.

Rejected: plain Chrome ids, with the CLI asking every host for the owner, because two browsers can
hand out the same number, and every call costs a round trip to every host.

Rejected: sending a command to every host, because on a clash between two browsers it acts in both.

Rejected: the shortest unique prefix of the profile id, because hosts come and go: an id printed
while two profiles ran could name a different profile after one stopped.
