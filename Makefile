
ifndef encoders
encoders = h264
endif

tags = 
ifneq (,$(findstring h264,$(encoders)))
tags = h264enc
endif

ifneq (,$(findstring vp8,$(encoders)))
tags := $(tags) vp8enc
endif

tags := $(strip $(tags))

agent.tar.gz: clean agent
	@tar zcf agent.tar.gz web agent

agent.zip: clean agent
	@zip -r agent.zip web agent

agent:
	go build -tags "$(tags)" cmd/agent.go

.PHONY: clean
clean:
	@if [ -f agent ]; then rm agent; fi
	@if [ -f agent.tar.gz ]; then rm agent.tar.gz ; fi
	@if [ -f agent.zip ]; then rm agent.zip ; fi
