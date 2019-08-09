

agent.tar.gz: agent
	@tar zcf agent.tar.gz web agent

agent.zip: agent
	@zip -r agent.zip web agent

agent:
	go build cmd/agent.go