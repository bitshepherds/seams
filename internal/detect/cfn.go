package detect

import (
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

// cfnResources are CloudFormation's topics, buses, streams and queues, with
// the property that names each.
var cfnResources = map[string]struct{ kind, prop string }{
	"AWS::SNS::Topic": {"event", "TopicName"}, "AWS::Events::EventBus": {"event", "Name"},
	"AWS::Kinesis::Stream": {"event", "Name"}, "AWS::SQS::Queue": {"queue", "QueueName"},
}

var (
	cfnGateways = map[string]bool{
		"AWS::ApiGateway::RestApi": true, "AWS::ApiGatewayV2::Api": true,
		"AWS::Serverless::Api": true, "AWS::Serverless::HttpApi": true,
	}
	cfnFunctions = map[string]bool{"AWS::Serverless::Function": true, "AWS::Lambda::Function": true}
	// samEvents maps a SAM function event's type to the property naming
	// what it receives from.
	samEvents = map[string]string{
		"SQS": "Queue", "SNS": "Topic", "Kinesis": "Stream", "EventBridgeRule": "EventBusName", "CloudWatchEvent": "EventBusName",
	}
)

// stack is one CloudFormation, SAM or Serverless template being read.
type stack struct {
	file, comp string
	resources  map[string]resource // logical id → topic or queue
	functions  map[string]string   // logical id → the component it runs
}

// templates reads CloudFormation, SAM and Serverless templates.
func (s *scan) templates() {
	for _, f := range s.filesNamed("*.yaml", "*.yml", "*.json", "*.template") {
		serverless := strings.HasPrefix(path.Base(f), "serverless.")
		if s.ci(f) || !serverless && !strings.Contains(s.read(f), "AWS::") {
			continue
		}
		root := s.yamlRoot(f)
		t := &stack{file: f, comp: s.component(f), resources: map[string]resource{}, functions: map[string]string{}}
		res := mapGet(root, "Resources")
		if serverless {
			res = mapGet(mapGet(root, "resources"), "Resources")
		}
		s.cfnResources(t, res)
		if serverless {
			s.serverlessFunctions(t, root)
		}
		s.cfnOutputs(t, mapGet(root, "Outputs"))
		s.importValues(t, root)
	}
}

// each calls fn with each key and value of a mapping.
func each(n *yaml.Node, fn func(k, v *yaml.Node)) {
	if n == nil || n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		fn(n.Content[i], n.Content[i+1])
	}
}

// scalar is a node's literal value, or empty for anything else, such as a
// function or a Serverless variable.
func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode || strings.HasPrefix(n.Tag, "!") && n.Tag != "!!str" || strings.Contains(n.Value, "${") {
		return ""
	}
	return n.Value
}

// refID is the logical id a Ref or Fn::GetAtt names, in either form.
func refID(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	switch n.Tag {
	case "!Ref":
		return n.Value
	case "!GetAtt":
		if n.Kind == yaml.SequenceNode && len(n.Content) > 0 {
			return n.Content[0].Value
		}
		return strings.SplitN(n.Value, ".", 2)[0]
	}
	if v := mapGet(n, "Ref"); v != nil {
		return v.Value
	}
	if v := mapGet(n, "Fn::GetAtt"); v != nil {
		return refID(&yaml.Node{Kind: v.Kind, Tag: "!GetAtt", Value: v.Value, Content: v.Content})
	}
	return ""
}

func (s *scan) cfnResources(t *stack, res *yaml.Node) {
	each(res, func(id, r *yaml.Node) {
		typ, props := mapGet(r, "Type"), mapGet(r, "Properties")
		if typ == nil {
			return
		}
		if def, ok := cfnResources[typ.Value]; ok {
			name := slug(scalar(mapGet(props, def.prop)))
			if name == "" {
				name = kebab(id.Value)
			}
			t.resources[id.Value] = resource{def.kind, name}
			s.own(def.kind, t.comp, name, low, s.evidence(sigIaCResource, owns, t.comp, t.file, typ.Line))
		}
		if cfnFunctions[typ.Value] {
			t.functions[id.Value] = s.functionComponent(t, id.Value, props)
		}
	})
	each(res, func(id, r *yaml.Node) {
		typ, props := mapGet(r, "Type"), mapGet(r, "Properties")
		switch {
		case typ == nil:
		case cfnGateways[typ.Value]:
			s.provide("http", t.comp, generic, medium, s.evidence(sigIaCGateway, provides, t.comp, t.file, typ.Line))
			s.facts[len(s.facts)-1].public = true
		case cfnFunctions[typ.Value]:
			s.samFunction(t, t.functions[id.Value], props)
		case typ.Value == "AWS::Lambda::EventSourceMapping":
			s.cfnDelivery(t, mapGet(props, "EventSourceArn"), mapGet(props, "FunctionName"), typ.Line)
		case typ.Value == "AWS::SNS::Subscription":
			s.cfnDelivery(t, mapGet(props, "TopicArn"), mapGet(props, "Endpoint"), typ.Line)
		}
	})
}

// functionComponent is the component a function's code is in: where its
// CodeUri points, or the one its logical id names, or the template's.
func (s *scan) functionComponent(t *stack, id string, props *yaml.Node) string {
	if uri := scalar(mapGet(props, "CodeUri")); uri != "" {
		if c, ok := s.componentHolding(path.Clean(path.Join(path.Dir(t.file), uri))); ok {
			return c
		}
	}
	if c, ok := s.named(kebab(id)); ok {
		return c
	}
	return t.comp
}

// samFunction reads a SAM function's events and environment.
func (s *scan) samFunction(t *stack, comp string, props *yaml.Node) {
	each(mapGet(props, "Events"), func(_, e *yaml.Node) {
		typ, eprops := mapGet(e, "Type"), mapGet(e, "Properties")
		switch prop, ok := samEvents[scalar(typ)]; {
		case ok:
			if r, found := t.resources[refID(mapGet(eprops, prop))]; found {
				s.receive(r, comp, s.evidenceFor(sigServerless, comp, t.file, typ.Line))
			}
		case scalar(typ) == "Api" || scalar(typ) == "HttpApi":
			s.publicAPI(comp, s.evidence(sigServerless, provides, comp, t.file, typ.Line))
		}
	})
	s.cfnEnv(t, comp, mapGet(mapGet(props, "Environment"), "Variables"))
}

// cfnDelivery: an event-source mapping or subscription makes the function it
// delivers to a receiver.
func (s *scan) cfnDelivery(t *stack, source, target *yaml.Node, line int) {
	r, ok := t.resources[refID(source)]
	comp, isFn := t.functions[refID(target)]
	if ok && isFn {
		s.receive(r, comp, s.evidenceFor(sigIaCSubscription, comp, t.file, line))
	}
}

// receive records a party receiving from a topic or queue.
func (s *scan) receive(r resource, party string, evidence func(role string) Evidence) {
	s.messageFact(r.kind, r.name, receiving, party, medium, evidence)
}

// evidenceFor makes the evidence at a line for whichever role it turns out
// to take.
func (s *scan) evidenceFor(signal, party, file string, line int) func(role string) Evidence {
	return func(role string) Evidence { return s.evidence(signal, role, party, file, line) }
}

// publicAPI records a component providing an HTTP API to the public.
func (s *scan) publicAPI(comp string, ev Evidence) {
	s.provide("http", comp, generic, medium, ev)
	s.facts[len(s.facts)-1].public = true
}

// cfnEnv records what a function's environment sets each name to.
func (s *scan) cfnEnv(t *stack, comp string, vars *yaml.Node) {
	each(vars, func(k, v *yaml.Node) {
		if r, ok := t.resources[refID(v)]; ok {
			s.setEnv(comp, k.Value, envTarget{res: &r, ev: s.evidence(sigEnv, "", comp, t.file, k.Line)})
		} else if u := scalar(v); strings.HasPrefix(u, "http") {
			s.setEnv(comp, k.Value, envTarget{url: u, ev: s.evidence(sigEnv, "", comp, t.file, k.Line)})
		}
	})
}

// serverlessFunctions reads a Serverless service's functions: their events
// and environments, and the provider's environment, which each inherits.
func (s *scan) serverlessFunctions(t *stack, root *yaml.Node) {
	s.cfnEnv(t, t.comp, mapGet(mapGet(root, "provider"), "environment"))
	each(mapGet(root, "functions"), func(_, fn *yaml.Node) {
		s.cfnEnv(t, t.comp, mapGet(fn, "environment"))
		events := mapGet(fn, "events")
		if events == nil || events.Kind != yaml.SequenceNode {
			return
		}
		for _, e := range events.Content {
			each(e, func(k, v *yaml.Node) { s.serverlessEvent(t, k, v) })
		}
	})
}

// serverlessEvents maps a Serverless event to the kind it receives, and the
// key that holds its source when the event is a mapping.
var serverlessEvents = map[string]struct{ kind, key string }{
	"sqs": {"queue", "arn"}, "sns": {"event", "arn"}, "stream": {"event", "arn"}, "eventBridge": {"event", "eventBus"},
}

func (s *scan) serverlessEvent(t *stack, k, v *yaml.Node) {
	ev := s.evidenceFor(sigServerless, t.comp, t.file, k.Line)
	if k.Value == "http" || k.Value == "httpApi" {
		s.publicAPI(t.comp, ev(provides))
		return
	}
	def, ok := serverlessEvents[k.Value]
	if !ok {
		return
	}
	src := v
	if v.Kind == yaml.MappingNode {
		src = mapGet(v, def.key)
		if src == nil && k.Value == "sns" {
			src = mapGet(v, "topicName")
		}
	}
	if r, found := t.resources[refID(src)]; found {
		s.receive(r, t.comp, ev)
	} else if name := scalar(src); name != "" {
		s.receive(resource{def.kind, slug(name[strings.LastIndexAny(name, ":/")+1:])}, t.comp, ev)
	}
}

// cfnOutputs remembers each export, so an import elsewhere can name its
// provider.
func (s *scan) cfnOutputs(t *stack, outputs *yaml.Node) {
	each(outputs, func(_, o *yaml.Node) {
		if name := scalar(mapGet(mapGet(o, "Export"), "Name")); name != "" {
			s.exports[name] = t.comp
		}
	})
}

// imported is a value a stack imports, to be matched to its export.
type imported struct {
	export string
	f      fact
}

// importValues reads each Fn::ImportValue: a seam to the stack exporting the
// value, which may be another repo's.
func (s *scan) importValues(t *stack, n *yaml.Node) {
	if n == nil {
		return
	}
	add := func(v *yaml.Node) {
		if name := scalar(&yaml.Node{Kind: v.Kind, Value: v.Value}); name != "" {
			s.imports = append(s.imports, imported{name, fact{kind: "config", name: slug(name), party: t.comp, conf: low,
				ev: s.evidence(sigRemoteState, consumes, t.comp, t.file, v.Line)}})
		}
	}
	if n.Tag == "!ImportValue" {
		add(n)
	}
	for i, c := range n.Content {
		if n.Kind == yaml.MappingNode && i%2 == 0 && c.Value == "Fn::ImportValue" {
			add(n.Content[i+1])
		}
		s.importValues(t, c)
	}
}
