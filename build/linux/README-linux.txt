Word Projector for Linux
========================
Run ./WordProjector. It needs GTK 3 and WebKitGTK 4.1, which most desktop
distributions already have (Debian/Ubuntu: libwebkit2gtk-4.1-0, Fedora: webkit2gtk4.1).

Headless mode (control from a browser): ./WordProjector --headless --listen 0.0.0.0:8777
Data directory: ~/.local/share/WordProjector (override with --data <dir>).

The first time it starts it puts a launcher (wordprojector.desktop) and its icon in ~/.local/share, so the dock and the
application menu show the Word Projector icon. It renews the launcher when the folder is moved; a launcher you edit
yourself (remove the line X-WordProjector-Launcher=1) is left alone.
