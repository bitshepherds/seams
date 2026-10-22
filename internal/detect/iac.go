package detect

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

// Signals read from infrastructure as code.
const (
	sigIaCResource     = "iac-resource"
	sigIaCGateway      = "iac-gateway"
	sigIaCSubscription = "iac-subscription"
	sigRemoteState     = "remote-state"
	sigServerless      = "serverless"
)

// envTarget is what IaC sets an env name to: a topic or queue, another
// component's endpoint, or a URL.
type envTarget struct {
	res  *resource
	comp string
	url  string
	ev   Evidence
}

// resource is a topic, queue or bus that IaC declares.
type resource struct{ kind, name string }

// tfResources are Terraform's topics, buses and queues, with the attribute
// that names each.
var tfResources = map[string]struct{ kind, attr string }{
	"aws_sns_topic": {"event", "name"}, "aws_cloudwatch_event_bus": {"event", "name"},
	"aws_kinesis_stream": {"event", "name"}, "google_pubsub_topic": {"event", "name"},
	"azurerm_servicebus_topic": {"event", "name"}, "confluent_kafka_topic": {"event", "topic_name"},
	"kafka_topic":   {"event", "name"},
	"aws_sqs_queue": {"queue", "name"}, "google_cloud_tasks_queue": {"queue", "name"},
	"azurerm_servicebus_queue": {"queue", "name"},
}

var (
	tfGateways = map[string]bool{
		"aws_api_gateway_rest_api": true, "aws_apigatewayv2_api": true, "google_api_gateway_api": true,
		"azurerm_api_management_api": true, "kubernetes_ingress_v1": true, "kubernetes_ingress": true,
	}
	tfCompute = map[string]bool{
		"aws_lambda_function": true, "aws_ecs_task_definition": true, "aws_ecs_service": true,
		"kubernetes_deployment": true, "kubernetes_deployment_v1": true, "google_cloud_run_service": true,
		"google_cloud_run_v2_service": true, "aws_apprunner_service": true,
	}
	tfSubscriptions = map[string]bool{
		"aws_sns_topic_subscription": true, "aws_lambda_event_source_mapping": true,
		"aws_cloudwatch_event_target": true, "google_pubsub_subscription": true,
	}

	tfHeader  = regexp.MustCompile(`^\s*(resource|data)\s+"([\w-]+)"\s+"([\w-]+)"\s*\{`)
	tfAttr    = regexp.MustCompile(`^\s*(\w+)\s*=\s*"([^"$]*)"`)
	tfRef     = regexp.MustCompile(`\b([a-z][a-z0-9_]*)\.([\w-]+)\.\w+`)
	tfEnv     = regexp.MustCompile(`^\s*"?([A-Z][A-Z0-9_]*)"?\s*=\s*(.+)$`)
	tfECSEnv  = regexp.MustCompile(`name\s*=\s*"([A-Z][A-Z0-9_]*)"\s*,?\s*value\s*=\s*([^,}]+)`)
	tfURL     = regexp.MustCompile(`"(https?://[^"]+)"`)
	tfPath    = regexp.MustCompile(`^\s*(?:filename|source_dir|source_file|source_path|working_dir)\s*=\s*"(?:\$\{path\.\w+\}/)?([^"$]+)"`)
	tfStrings = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|#.*|//.*`)
)

// tfBlock is a Terraform resource or data block.
type tfBlock struct {
	kind, typ, label, file string
	line                   int      // the header's
	body                   []string // its lines, the header first
}

func (b tfBlock) handle() string { return b.typ + "." + b.label }

// attr is the first of the named attributes the block sets to a literal.
func (b tfBlock) attr(names ...string) string {
	for _, l := range b.body[1:] {
		if m := tfAttr.FindStringSubmatch(l); m != nil {
			for _, n := range names {
				if m[1] == n {
					return m[2]
				}
			}
		}
	}
	return ""
}

// refs lists the blocks of a type that the block refers to, by label.
func (b tfBlock) refs(types map[string]bool) []string {
	var out []string
	for _, l := range b.body[1:] {
		for _, m := range tfRef.FindAllStringSubmatch(l, -1) {
			if types[m[1]] {
				out = append(out, m[1]+"."+m[2])
			}
		}
	}
	return out
}

// tfBlocks reads a Terraform file's top-level resource and data blocks,
// counting braces outside strings and comments.
func (s *scan) tfBlocks(f string) []tfBlock {
	var blocks []tfBlock
	var cur *tfBlock
	depth := 0
	for i, l := range s.lines(f) {
		if cur == nil {
			m := tfHeader.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			cur, depth = &tfBlock{kind: m[1], typ: m[2], label: m[3], file: f, line: i + 1}, 0
		}
		cur.body = append(cur.body, l)
		bare := tfStrings.ReplaceAllString(l, "")
		depth += strings.Count(bare, "{") - strings.Count(bare, "}")
		if depth <= 0 {
			blocks = append(blocks, *cur)
			cur = nil
		}
	}
	return blocks
}

func (s *scan) iacSignals() {
	var blocks []tfBlock
	for _, f := range s.filesNamed("*.tf") {
		blocks = append(blocks, s.tfBlocks(f)...)
	}
	resources, computes := map[string]resource{}, map[string]string{}
	for _, b := range blocks {
		if r, ok := tfResources[b.typ]; ok && b.kind == "resource" {
			name := slug(b.attr(r.attr))
			if name == "" {
				name = slug(b.label)
			}
			resources[b.handle()] = resource{r.kind, name}
			s.own(r.kind, s.component(b.file), name, low, s.evidence(sigIaCResource, owns, s.component(b.file), b.file, b.line))
		}
		if tfCompute[b.typ] {
			computes[b.handle()] = s.computeComponent(b)
		}
	}
	for _, b := range blocks {
		switch {
		case tfCompute[b.typ]:
			s.tfEnv(b, computes[b.handle()], resources, computes)
		case tfGateways[b.typ]:
			s.tfGateway(b, blocks, computes)
		case tfSubscriptions[b.typ]:
			s.tfSubscription(b, resources, computes)
		case b.kind == "data" && b.typ == "terraform_remote_state":
			comp := s.component(b.file)
			s.consume("config", slug(b.label), "state", comp, medium, s.evidence(sigRemoteState, consumes, comp, b.file, b.line))
			s.facts[len(s.facts)-1].note = "a seam to another repo, through its Terraform state"
		}
	}
	s.templates()
}

// computeComponent is the component a Lambda function, container or
// deployment runs: the one its code's path, such as filename, points into,
// or the one its label or name names, or empty.
func (s *scan) computeComponent(b tfBlock) string {
	for _, l := range b.body[1:] {
		if m := tfPath.FindStringSubmatch(l); m != nil {
			if c, ok := s.componentHolding(path.Clean(path.Join(path.Dir(b.file), m[1]))); ok {
				return c
			}
		}
	}
	for _, n := range []string{b.attr("function_name", "family", "name"), b.label} {
		if c, ok := s.named(slug(n)); ok {
			return c
		}
	}
	return ""
}

// componentHolding is the component, other than the root, whose directory
// holds a path, whether or not the repo has it, such as a build's zip.
func (s *scan) componentHolding(p string) (string, bool) {
	best := Component{}
	for _, c := range s.comps {
		if c.Dir != "." && (p == c.Dir || within(p, c.Dir)) && len(c.Dir) > len(best.Dir) {
			best = c
		}
	}
	return best.Name, best.Name != ""
}

// tfEnv records what a compute block sets each env name to.
func (s *scan) tfEnv(b tfBlock, comp string, resources map[string]resource, computes map[string]string) {
	if comp == "" {
		return
	}
	for i, l := range b.body[1:] {
		m := tfECSEnv.FindStringSubmatch(l)
		if m == nil {
			m = tfEnv.FindStringSubmatch(l)
		}
		if m == nil {
			continue
		}
		t := envTarget{ev: s.evidence(sigEnv, "", comp, b.file, b.line+i+1)}
		if u := tfURL.FindStringSubmatch(m[2]); u != nil {
			t.url = u[1]
		}
		if r := tfRef.FindStringSubmatch(m[2]); r != nil {
			h := r[1] + "." + r[2]
			if res, ok := resources[h]; ok {
				t.res = &res
			}
			t.comp = computes[h]
		}
		if t.res != nil || t.url != "" || t.comp != "" {
			s.setEnv(comp, m[1], t)
		}
	}
}

func (s *scan) setEnv(comp, name string, t envTarget) {
	if s.envMap[comp] == nil {
		s.envMap[comp] = map[string]envTarget{}
	}
	s.envMap[comp][name] = t
}

// tfGateway: a gateway or ingress makes what it fronts a public HTTP
// provider: the one component it, or the blocks naming it, such as its
// integrations, route to; or failing that, its file's component.
func (s *scan) tfGateway(b tfBlock, blocks []tfBlock, computes map[string]string) {
	fronted := map[string]bool{}
	for _, other := range blocks {
		if other.handle() == b.handle() || strings.Contains(strings.Join(other.body, "\n"), b.handle()+".") {
			for _, h := range other.refs(tfCompute) {
				if c := computes[h]; c != "" {
					fronted[c] = true
				}
			}
		}
	}
	comp := s.component(b.file)
	if len(fronted) == 1 {
		for c := range fronted {
			comp = c
		}
	}
	s.provide("http", comp, generic, medium, s.evidence(sigIaCGateway, provides, comp, b.file, b.line))
	s.facts[len(s.facts)-1].public = true
}

// tfSubscription: a subscription or event-source mapping makes the compute
// it delivers to a receiver of the topic or queue.
func (s *scan) tfSubscription(b tfBlock, resources map[string]resource, computes map[string]string) {
	var res *resource
	body := strings.Join(b.body, "\n")
	for _, h := range sortedKeys(resources) {
		if r := resources[h]; strings.Contains(body, h+".") {
			res = &r
			break
		}
	}
	var comp string
	if hs := b.refs(tfCompute); len(hs) > 0 {
		comp = computes[hs[0]]
	}
	if res == nil || comp == "" {
		return
	}
	s.messageFact(res.kind, res.name, receiving, comp, medium, func(role string) Evidence {
		return s.evidence(sigIaCSubscription, role, comp, b.file, b.line)
	})
}

// sortedKeys is a map's keys, in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
