MKDIR='/bin/mkdir'
CHOWN='/bin/chown'
CHMOD='/bin/chmod'
CP='/bin/cp'
RM='/bin/rm'
SYSTEMCTL='/bin/systemctl'
GO='/usr/local/go/bin/go'

build:
	$(GO) build -o build/b2s
install:
	$(CP) build/b2s /usr/local/bin/b2s
	$(CP) emoji_pretty.json /etc/b2s/

	# Install suristats_consumer service
	$(CP) b2s.service /etc/systemd/system/
	$(SYSTEMCTL) enable b2s.service

	$(SYSTEMCTL) daemon-reload

clean:
	$(RM) -rf build/b2s

