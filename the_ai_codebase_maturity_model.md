The AI Codebase Maturity Model: From

Assisted Coding to Self-Sustaining

Systems

A Practitioner Experience Report from KubeStellar Console

Andy Anderson, Ph.D. (@clubanderson) IBM Research — Hybrid Cloud

and AI Platform

Dr. Anderson  holds  a  Ph.D.  in  Technology Adoption  and  has  been  with

IBM  for  9  years,  the  last  4  with  IBM  Research.  He  has  served  as  chief

maintainer  of  the  KubeStellar  organization  for  4  years,  stewarding  it

through 3 years as a CNCF Sandbox project.

Transparency note: This paper was written with AI assistance. The author

was  interviewed  by  Claude  Code  (Anthropic’s  Claude  Opus),  and  the

content was organized with AI support. The 4-month experience described

herein is entirely the author’s — every architectural decision, every failure,

every insight came from hands-on daily work with AI coding agents. Using

AI to write about AI-driven development felt appropriate, and honestly, it

would have been strange not to.

Abstract

AI  coding  tools  are  widely  adopted,  but  most  teams  plateau  at  “prompt

and review” without a framework for systematic progression. This paper

presents the AI Codebase Maturity Model (ACMM), a 5-level framework

describing how codebases evolve from basic AI-assisted coding to self-

sustaining  systems.  Inspired  by  CMMI,  each  level  is  defined  by  its

feedback  loop  topology  —  the  specific  mechanisms  that  must  exist

before the next level becomes possible. I validate the model through a 4-

month  experience  report  maintaining  KubeStellar  Console,  a  CNCF

Kubernetes  dashboard  built  from  scratch  with  Claude  Code  (Opus)  and

GitHub  Copilot.  The  system  currently  operates  with  63  CI/CD

workflows, 32 nightly test suites, 91% code coverage, and achieves bug-

to-fix times under 30 minutes — 24 hours a day. The central finding: the

intelligence  of  an  AI-driven  development  system  resides  not  in  the  AI

model  itself,  but  in  the  infrastructure  of  instructions,  tests,  metrics,  and

feedback loops that surround it. You cannot skip levels, and at each level,

the  thing  that  unlocks  the  next  one  is  another  feedback  mechanism.

Testing  —  the  volume  of  test  cases,  the  coverage  thresholds,  and  the

reliability  of  test  execution  —  proved  to  be  the  single  most  important

investment in the entire journey.

1. Introduction

In  December  2025,  I  started  building  KubeStellar  Console  —  a

Kubernetes  multi-cluster  management  dashboard  —  from  scratch  using

AI  coding  agents.  Full  stack.  Go  backend,  React/TypeScript  frontend,

Helm charts, CI/CD, the works. I had no team. Just me and Claude Code.

The  first  two  weeks  were  exhilarating.  Code  poured  out  at  a  pace  I’d

never  experienced.  Features  that  would  normally  take  days  appeared  in

hours.  It  felt  like  having  a  tireless  junior  developer  who  typed  at  the

speed of thought.

Then the limitations hit. All at once.

Broken  builds.  Wrong  architectural  patterns.  Scope  creep  —  the  AI

modifying files I didn’t ask it to touch. And the cascade problem: fix one

thing,  three  others  break.  I  was  spending  more  time  reviewing  and

reverting  than  I  would  have  spent  writing  the  code  myself.  The  “10x

productivity” promise started feeling like a net negative.

This  experience  —  fast  initial  satisfaction  followed  by  systemic

frustration  —  appears  to  be  universal  among  developers  adopting  AI

coding  tools.  The  tools  are  capable,  but  capability  without  structure

produces chaos. What was missing was not a better AI model. What was

missing  was  a  systematic  approach  to  making  the AI  effective  within  a

real codebase over time.

Several  maturity  models  for  AI-assisted  development  have  emerged  in

2025-2026 (Section 2.3), but they share a common orientation: defining

maturity  by  how  much  autonomy  AI  agents  have  —  how  much  the

human steps back. This paper takes a different approach.

The AI Codebase Maturity Model (ACMM) is a 5-level framework that

defines maturity not by autonomy, but by feedback loop topology — the

specific mechanisms through which a codebase measures, adapts to, and

governs the behavior of AI agents. Drawing inspiration from CMMI [1],

the model argues that each level depends on infrastructure established at

the  previous  level.  You  cannot  skip  levels.  And  the  thing  that  unlocks

each  transition  is  not  giving  AI  more  freedom  —  it  is  adding  another

feedback mechanism.

The contributions of this paper are:

1. The ACMM framework — five levels with defined

characteristics, transition triggers, and anti-patterns, applicable

regardless of specific AI tooling.

2. A longitudinal case study — 4 months of daily AI-assisted

development, from zero to a self-sustaining system with

quantitative metrics at each level.

3. Practitioner guidance — specific, actionable recommendations

for progressing through each level transition.

Have you ever cloned an open source project, used it for a few minutes,

and realized it didn’t quite solve your challenge? Or waited weeks to find

out  if  a  maintainer  would  even  look  at  your  issue?  By  Level  5  of  this

model, those experiences become relics. The system I describe processes

bug  reports

to  fixes

in  under  30  minutes,  feature  requests

to

implementations  in  under  60  minutes,  24  hours  a  day.  This  is  not

theoretical. It is running today.

The  paper  is  organized  as  follows:  Section  2  reviews  related  work  in

maturity  models  and  AI-assisted  development.  Section  3  presents  the

ACMM  framework.  Section  4  validates

the  model

through

the

KubeStellar  Console  case  study.  Section  5  discusses  key  insights  and

limitations. Section 6 offers implications for practitioners, and Section 7

concludes.

2. Background and Related Work

2.1 Capability Maturity Models

The Capability Maturity Model (CMM) was introduced by the Software

Engineering  Institute  in  1993  [1]  to  provide  organizations  with  a

roadmap  for  software  process  improvement.  Its  successor,  CMMI  [2],

extended  the  framework  across  disciplines.  The  core  insight  —  that

process  maturity  follows  predictable,  sequential  stages,  and  that  each

stage  depends  on  capabilities  established  at  the  previous  stage  —  has

proven durable despite criticism of rigid level definitions [3].

DevOps adopted similar thinking through the DORA metrics framework

[4], which identifies four key metrics (deployment frequency, lead time,

change  failure  rate,  mean

time

to  restore)

that  correlate  with

organizational  performance.  The  DORA  model  demonstrated  that

maturity  frameworks  remain  useful  when  grounded  in  measurable

outcomes rather than prescriptive processes.

2.2 AI-Assisted Software Development

The landscape of AI coding tools has expanded rapidly. GitHub Copilot

[5],  Claude  Code  [6],  Cursor,  and  autonomous  agents  like  SWE-Agent

[7]  represent  a  spectrum  from  suggestion-based  assistance  to  fully

autonomous  coding.  Empirical  studies  have  shown  productivity

improvements:  Peng  et  al.  [8]  found  that  Copilot  users  completed  tasks

55.8%  faster;  Ziegler  et  al.  [9]  found  that  suggestion  acceptance  rate  is

the  primary  driver  of  developers’  perceived  productivity  with  AI  code

completion.

However, these studies focus on individual task completion. They do not

address  how  teams  or  codebases  systematically  improve  their  AI

integration over time. A developer who is 55% faster at generating code

but spends equivalent time debugging AI-introduced regressions has not

achieved a net improvement.

2.3 Emerging AI Development Maturity Models

Several  frameworks  addressing  AI-assisted  development  maturity  have

appeared in 2025-2026:

Dan Shapiro’s Five Levels [14] maps progression from Manual (Level

0)  to  the  “Dark  Factory”  (Level  5),  borrowing  from  the  automotive

industry’s  autonomous  driving  framework.  The  model  focuses  on

autonomy — how much can AI do without human involvement? — and

has been widely adopted in industry discourse [15, 16].

The AI-SDLC Maturity Model [17] (ELEKS) defines five stages from

Traditional to AI-Autonomous development, with emphasis on enterprise

governance and the organizational changes required at each stage.

The AI  Development  Maturity  Model  (AIDMM)  [18]  describes  five

levels  from  purely  human  coding  to  fully  autonomous  AI-driven

codebases,  focusing  on  how  the  developer’s  role  evolves  from  writing

code to orchestrating agents.

AI-MM SET [19] (Gigacore) introduces a three-axis scoring system —

Autonomy,  Controls,  and  Governance  —  arguing  that  higher  autonomy

without stronger controls is a risk, not progress.

Table 0: Comparison of AI Development Maturity Models

Defining
Axis

Scope

Levels

Validated
Case Study

Feedback
Loop Focus

Autonomy

Individual/team

6 (0-
5)

No
(conceptual)

No

SDLC
phase

maturity

Developer
role

evolution

Autonomy
× Controls

×
Governance

Enterprise

Individual

Team/org

Codebase

5

5

5

5

No
(consulting)

Governance-
focused

No
(conceptual)

No

No
(framework
only)

Controls

axis

Yes (4

months,
quantitative)

Central
thesis

ACMM

Feedback

(this
paper)

loop
topology

Model

Shapiro
/ Dark
Factory

[14]

AI-
SDLC

[17]

AIDMM
[18]

AI-MM
SET
[19]

These  frameworks  share  a  common  orientation:  they  define  maturity

primarily in terms of how much autonomy AI agents have, or how mature

the organization is in adopting AI. They ask: “How much can the AI do

without you?”

The ACMM occupies a distinct space: it asks “What must the codebase

encode before the next level of AI effectiveness is possible?” The unit of

analysis is the codebase and its surrounding infrastructure, not the team

or the organization. This makes the model applicable to solo maintainers

and large teams alike — the levels are properties of the system, not of the

people operating it.

2.4 The Gap

The  existing  models  are  valuable  contributions,  but  they  share  a

limitation: they are defined by the degree of human absence rather than

by  the  infrastructure  that  makes  AI  effective. A  team  that  gives AI  full

autonomy  without  measurement  is  not  mature  —  it  is  reckless.

Autonomy is an output of maturity, not a definition of it.

None of the existing frameworks are defined by feedback loop topology

— the specific mechanisms (tests, acceptance metrics, error monitoring,

self-tuning configurations) that must exist before the next level becomes

possible.  None  provide  a  validated  case  study  with  quantitative  metrics

across all levels. And none address the concept of the codebase as model

— where the code patterns, instruction files, and test suites collectively

teach AI agents how to operate, making the repository itself the primary

source of AI intelligence rather than the underlying language model.

The  ACMM  fills  this  gap  by  defining  maturity  in  terms  of  what  the

codebase  measures,  encodes,  and  enforces  —  not  in  terms  of  what  the

human stops doing.

3. The AI Codebase Maturity Model

The ACMM rests on three design principles:

1. Each level subsumes the previous. You cannot skip levels. Level

4 requires Level 3’s measurement infrastructure, which requires

Level 2’s consistency, which requires Level 1’s basic capability.

2. Levels are defined by feedback loop topology, not by tool

choice. The same model applies whether you use Claude, Copilot,

or any future AI coding tool.

3. Progression is measured by the degree to which human

judgment is encoded in system artifacts — instruction files,

tests, metrics, workflow rules — rather than held in human

memory.

Level 1 — Assisted: Prompt and Review

Characteristic:  The  human

initiates  every

interaction.  AI

is  a

sophisticated  autocomplete.  No  persistent  context  exists  between

sessions.

Typical  practices: Ad-hoc  prompting,  manual  review  of  all AI  output,

copy-paste  from  chat  interfaces,  one-off  code  suggestions  accepted  or

rejected in isolation.

Key  artifacts:  None  beyond  the  code  itself.  All  knowledge  about

preferences, patterns, and architectural decisions lives in the developer’s

head.

Transition trigger: The developer notices they keep repeating the same

corrections  across  sessions  and  wishes  the  AI  “just  knew”  their

preferences.

Anti-pattern: “I tried Copilot but it doesn’t understand our codebase” —

blaming the tool for the absence of encoded context.

Level 2 — Instructed: Encoded Preferences

Characteristic: Preferences, conventions, and architectural decisions are

written  into  files  that  AI  agents  consume  at  the  start  of  every  session.

Output becomes consistent across sessions and agents.

Typical  practices:  CLAUDE.md
.github/copilot-instructions.md   with
conventions,  development  guides  encoding  common  rejection  reasons,

files  with

commit

project

rules,

and

PR

structured PR templates with AI-readable checklists.

Key  artifacts:  Instruction  files,  codified  style  guides,  card/component

development  guides.  In  my  case,  a  single  card  development  guide

encoded  approximately  90%  of  the  reasons  I  had  been  rejecting  AI-

generated PRs.

Transition  trigger:  The  developer  realizes

instructions

improve

consistency  but  cannot  tell  them  whether  the AI  is  actually  performing

well. Gut feeling replaces data. They want measurement.

Anti-pattern: Ever-growing instruction files with contradictory rules, no

mechanism to determine which instructions are effective.

Level 3 — Measured: Feedback Becomes Visible

Characteristic: The system produces quantitative signals about AI agent

performance.  Acceptance  rates,  coverage  metrics,  error  rates,  and  user

feedback are tracked systematically.

Typical  practices:  PR  acceptance  rate  tracking  per  category,  code

coverage  gating  on  every  PR,  nightly

test  suites  (compliance,

performance,

security,  accessibility),  GA4  or  equivalent  error

monitoring, NPS surveys for user sentiment, weekly flaky test analysis.

Key  artifacts:  Acceptance  rate  logs  (e.g.,  auto-qa-tuning.json ),
coverage reports, monitoring dashboards, error classification systems.

Testing  as  the  breakthrough:  This  is  where  I  want  to  be  emphatic.

Testing  —  the  sheer  volume  of  test  cases,  the  coverage  thresholds,  and

critically  the  reliability  of  test  execution  —  was  the  single  most

important investment in the entire journey. At Level 2, I had instructions

that  made  the  AI  consistent.  At  Level  3,  I  had  tests  that  made  the  AI

trustworthy. A flaky test in a human workflow is annoying. A flaky test

in  an  autonomous  workflow  is  dangerous  —  it  erodes  the  trust  that

enables  every  subsequent  level.  The  compounding  effect  of  volume,

coverage, and determinism cannot be overstated.

Transition trigger: The developer sees patterns in the data and realizes

certain responses should be automated — “why am I manually adjusting

weights when the data tells me what to do?”

Anti-pattern:  “Dashboard  graveyard”  —  metrics  collected  but  never

acted  upon.  The  human  remains  the  bottleneck  for  every  interpretation

and decision.

Level 4 — Adaptive: Feedback Loops Close Themselves

Characteristic: The  system  acts  on  its  own  metrics. Thresholds  trigger

automated  responses.  Human  oversight  shifts  from  execution

to

governance.

Typical practices: Self-tuning rotation weights based on PR acceptance

rates,  automated  issue  triage  loops  (every  15  minutes  across  multiple

repos),  overnight  autonomous  bug  fixes,  proactive  scanning  for  user

engagement  opportunities,  contributor  leaderboards  for  coordination,

error recovery with exponential backoff.

Key  artifacts:  Self-modifying  configuration  files  (like  auto-qa-
tuning.json ,  where  categories  with  acceptance  rates  below  20%  are
automatically  blocked),  closed-loop  CI/CD  pipelines,  worktree-based

concurrent AI sessions, automated content generation (MARP slides with

ElevenLabs narration, documentation sync from PR screenshots).

Transition trigger: The developer realizes the system’s behavior is now

primarily  determined  by  its  artifacts  —  tests,  configs,  workflows  —

rather than by real-time human decisions. The code is the policy.

Anti-pattern:  Autonomous  action  without  sufficient  guardrails.  The

system  optimizes  for  a  proxy  metric  and  drifts  from  user  value. This  is

why  the  transition  from  Level  3  is  critical  —  you  need  measurement

before you can safely automate.

Level 5 — Self-Sustaining: Code Is the Model

Characteristic: Instructions encode human judgment. Tests encode trust

boundaries. Metrics encode priorities. The community steers the project

by  opening  issues;  AI  agents  implement  continuously.  The  system

operates 24/7 with human governance, not human execution.

Typical practices: Community-driven issue-to-implementation pipelines

(bug  to  fix  in  30  minutes,  feature  to  implementation  in  60  minutes),

automated  documentation

and

tutorial  generation,  multi-agent

orchestration  across  repositories,  self-improvement  cycles  where  the

system analyzes its own merged PRs and updates its guidance.

Key artifacts: The entire codebase is the AI’s operating manual. Every

test  is  a  trust  constraint.  Every  workflow  is  a  policy.  Every  metric

threshold  is  a  prioritization  decision.  The  instruction  files,  the  code

patterns,  the  acceptance  rate  history,  and  the  workflow  rules  form  a

coherent system that any AI agent can read and operate within — without

human presence at the keyboard.

At  this  level,  the  accumulated  size  of  the  codebase  —  the  tests,  the

instruction  files,  the  workflow  rules,  the  code  patterns  —  all  work  in

concert  not  just  to  resolve  issues,  but  to  evaluate  whether  an  issue  is

genuinely  a  bug  or  a  user  misunderstanding. A  concrete  example  from

April  2026:  a  user  filed  a  bug  reporting  that  a  Kubernetes  cluster  was

marked  “healthy”  while  pods  were  in  ImagePullBackOff  state.  The

system  responded  —  before  any  human  reviewed  it  —  explaining  that

cluster  health  reflects

infrastructure  health  (node  readiness,  API

reachability), which is architecturally separate from workload health (pod

status). The user didn’t have a bug. They had a misunderstanding of how

Kubernetes clusters work. The codebase knew the difference because the

tests,  the  health-check  logic,  and  the  documentation  all  encoded  that

design decision [see kubestellar/console#5475].

Anti-pattern:  Mistaking  autonomy  for  abandonment.  Level  5  requires

more  human  judgment,  not  less  —  but  the  judgment  is  applied  to

governance,  architecture,  and  values  rather  than  to  individual  lines  of

code. The human role shifts from typing to steering.

Table 1: ACMM Level Summary

Level Name

Feedback Loop

Key Artifacts

Human

Role

1

2

3

4

5

Assisted

Open loop

None

Executor

Instructed

One-way (human→AI)

Instruction
files

Rule-writer

Measured

Adaptive

Two-way (with human
interpretation)

Metrics, test
suites

Analyst

Closed loop (automated

Self-tuning

response)

configs

Self-

Multi-loop (self-

The codebase

sustaining

improving)

itself

Governor

Strategist

4. Case Study: KubeStellar Console

4.1 System Context

KubeStellar  Console

is  an  open-source  Kubernetes  multi-cluster

management dashboard, part of the KubeStellar project under the Cloud

Native Computing Foundation (CNCF) Sandbox. Development began in

mid-December 2025 with a single maintainer (the author) using Claude

Code with Opus for primary development and GitHub Copilot for code

review.

The  current  system  comprises  a  Go  backend,  a  React/TypeScript

frontend,  Helm  charts  for  deployment,  and  an  extensive  CI/CD

infrastructure.  It  is  maintained  across  4  repositories  with  multiple

concurrent  AI  coding  sessions  operating  via  git  worktrees.  The  full

source  code,  including  all  workflow  files,  instruction  files,  and  tuning

configurations referenced in this paper, is publicly available [12].

4.2 Evolution Through the Levels

Weeks 1-2 (Level 1 — Assisted): The Honeymoon

Full-stack development from scratch. Go API handlers, React dashboard,

WebSocket  connections,  Helm  packaging  —  all  generated  through

interactive AI sessions. Output was fast and exciting. But every session

started  fresh  with  no  memory  of  decisions  made  in  previous  sessions.

The same architectural mistakes recurred daily.

Weeks 3-4 (Level 2 — Instructed): Stopping the Bleeding

The  cascade  problem  (“fix  one  thing,  three  others  break”)  forced  the
creation  of  CLAUDE.md   —  initially  just  a  list  of  things  to  stop  doing.
“Don’t modify other people’s cards.” “Always run build and lint before

committing.”  “Use  named  constants,  not  magic  numbers.”  This  was
followed by  copilot-instructions.md  for PR conventions and a card
development guide encoding the top rejection reasons.

The  effect  was  immediate.  Consistency  across  sessions  improved.  The

same mistakes stopped recurring. But I had no way to know how well the

system was performing in aggregate.

Weeks 5-8 (Level 3 — Measured): The Testing Investment

This  was

the  real

turning  point.  I

invested  heavily

in

testing

infrastructure:  32  nightly  test  suites  covering  compliance,  performance,

dashboard  health,  nil  safety,  accessibility,  i18n,  and  visual  regression.

Code  coverage  reached  91%  across  12  parallel  shards. A  weekly  flaky

test analysis workflow identified unreliable tests and auto-created issues.

rate

The  acceptance
tracking  system  emerged  here:  auto-qa-
tuning.json ,  a  file  that  records  how  many  AI-generated  PRs  are
merged  vs.  closed  for  each  quality  category.  This  data  became  the

foundation for everything that followed.

One specific lesson: a Playwright E2E test for drag-and-drop interactions

passed  85%  of  the  time.  In  a  human  workflow,  that’s  tolerable.  In  an

autonomous workflow where test results gate PR merges, it meant good

fixes  were  randomly  blocked  and  bad  ones  randomly  approved.  I  spent

three  days  fixing  that  single  test  —  a  timing  issue  with  animation

completion  in  CI.  The  lesson  extended  far  beyond  drag-and-drop:  you

cannot have autonomous AI development without deterministic tests.

Weeks 9-12 (Level 4 — Adaptive): The Loops Close

With measurement in place, automation followed. The Auto-QA system

began  running  4  times  daily  with  8  layers  of  quality  checks.  PR

acceptance  rates  per  category  drove  automatic  weight  adjustments  —

accessibility  PRs  at  62%  acceptance  were  boosted  (0.93  weight),  while

operator  PRs  at  8%  acceptance  were  blocked  entirely  (0  weight).  The

system  learned  what  I  valued  without  me  manually  configuring  each

preference.

A  triage  loop  began  scanning  4  repositories  every  15  minutes.  A  PR

monitor  workflow  ran  every  60  seconds  checking  build  status.  Error

recovery workflows implemented exponential backoff for stuck agents. A

queue  processor  limited  concurrent  AI  sessions  to  2  to  prevent

interference.

GA4  error  monitoring  began  running  hourly,  querying  production

analytics  for  error  spikes  and  auto-creating  GitHub  issues.  Users  filing

bugs  through  the  dashboard’s  feedback  button  triggered  the  same

pipeline — issue creation, AI agent assignment, PR generation, preview

deployment, user notification.

A  contributor  leaderboard  was  added  that  serves  multiple  functions

beyond  simple  coordination.  First,  it  gamifies  contributions  —  public

recognition  in  a  point  system  incentivizes  participation.  Second,  it

provides visibility into our IFOS (Interns for Open Source) program —

we  can  gauge  in  real  time  whether  mentees  are  active  and  making

positive  impact.  Third,  and  perhaps  most  interestingly,  it  feeds  back  to

each contributor which components of the console they are focusing on

most and least. A contributor who sees they’ve spent 80% of their effort

on  one  area  can  consciously  spread  their  work  across  different

components. This helps prevent concentration — where an entire area of

the  codebase  gets  neglected  because  everyone  gravitates  to  the  same

module.

Weeks 13-16 (Level 5 — Self-Sustaining): Code Is the Model

The  transition  to  Level  5  was  not  a  single  event  but  a  recognition:  the

system’s  behavior  was  now  determined  by  its  artifacts,  not  by  my  real-

time presence. Issues filed at 2 AM were triaged, assigned, fixed, tested,

and ready for my review by 6 AM. The weekly self-improvement cycle

analyzed merged PRs and updated the Auto-QA’s own guidance — “keep

changes under 50 lines for best acceptance rate.”

Automated  tutorial  creation  was  added:  MARP  slides  generated  from

feature  descriptions,  narrated  by  ElevenLabs  (voice:  Rachel),  combined

with CDP-captured screenshots. Documentation sync workflows scanned

merged  PRs,  captured  UI  screenshots,  and  created  docs  PRs

automatically.

The result — and I do not say this lightly — is what I believe to be the

world’s

first

fully

integrated  and

fully  automated  Kubernetes

management  and  orchestration  solution.  Not  because  of  any  single

technology, but because all the pieces are wired together and they don’t

stop running when I go to bed.

4.3 Feedback Loop Inventory

The  following  table  enumerates  every  feedback  loop  operating  in  the

KubeStellar  Console  repository,  mapped  to  the  ACMM  level  where  it

was

introduced  and

the  specific  file  or  workflow  where

it

is

implemented.

Table 2: Complete Feedback Loop Inventory

Feedback

Loop

ACMM

Level

Repo Location

CLAUDE.md

instructions

Copilot

instructions

Card

development
guide

PR template
checklist

Build & lint
gate

Code coverage
gate (80%)

Full coverage
suite (12
shards)

Weekly flaky
test analysis

Nightly
compliance (10

suites)

Nightly

dashboard
health

Nil safety
scanner

Performance
TTFI gate

Playwright
E2E

Cross-browser
nightly

L2

L2

L2

L2

L3

L3

L3

L3

L3

L3

L3

L3

L3

L3

/CLAUDE.md

/.github/copilot-instructions.md

/.github/CARD_DEVELOPMENT_GUIDE.md

Frequency

Every AI

session

Every

Copilot
session

Every card
PR

/.github/pull_request_template.md

Every PR

/.github/workflows/copilot-build-

check.yml

Every push

/.github/workflows/coverage-gate.yml

Every PR

/.github/workflows/coverage-

hourly.yml

/.github/workflows/coverage-weekly-

review.yml

/.github/workflows/nightly-

compliance.yml

/.github/workflows/nightly-dashboard-

health.yml

/.github/workflows/nil-safety.yml

Hourly on
main

Weekly
(Monday

9am)

Daily 5am

UTC

Daily

4:45am
UTC

Daily
4:30am +
every PR

/.github/workflows/perf-ttfi.yml

Every PR

/.github/workflows/playwright.yml

/.github/workflows/playwright-

nightly.yml

Every push
to main

Daily
6:30am
UTC

Feedback

Loop

GA4 error
monitoring

ACMM

Level

L3

/.github/workflows/ga4-error-

monitor.yml

Repo Location

Frequency

NPS surveys

L3

/web/src/hooks/useNPSSurvey.ts

Hourly
(:42)

After 5th
session

Daily +
every PR

L3

L4

L4

L4

L4

L4

L4

L4

L4

L4

L4

CodeQL

security
analysis

Auto-QA (8
layers)

Auto-QA self-
tuning

PR acceptance

rate tracking

Copilot PR

monitor

Copilot error
recovery

Copilot retry
queue

Copilot build

monitor
(escalation)

Copilot review
auto-apply

Issue triage
loop

User feedback
→ GitHub
issues

Engagement
opportunity

scanning

Contributor

leaderboard

Automated
tutorial

generation

/.github/workflows/codeql.yml

/.github/workflows/auto-qa.yml

4x daily

/.github/auto-qa-tuning.json

Weekly

/.github/auto-qa-tuning.json

Continuous

/.github/workflows/copilot-pr-

monitor.yml

/.github/workflows/copilot-

recovery.yml

/.github/workflows/copilot-retry.yml

/.github/workflows/copilot-build-

monitor.yml

/.github/workflows/copilot-review-

apply.yml

Claude Code  /loop  (4 repos)

/pkg/api/handlers/feedback.go

Every 60

seconds

On failure
events

Every 5
minutes

On Netlify
failure

On review
submitted

Every 15
minutes

On user
submission

L4

Auto-QA Layer 8 (adoption psychology)

4x daily

L4

https://kubestellar.io/en/leaderboard

Continuous

L5

MARP + ElevenLabs pipeline

On feature

completion

Feedback
Loop

ACMM
Level

Repo Location

L5

CDP screenshots + docs PR

Automated

documentation
sync

Self-
improvement
analysis

L5

Auto-QA Layer 7 (weekly)

Weekly

Frequency

On

goodnight
trigger

Community
issue-to-

implementation

L5

Full pipeline (issue → triage → AI → PR → QA →
merge)

24/7

This inventory totals 33 distinct feedback loops. Each one was built to

solve  a  specific  problem.  Together,  they  form  the  nervous  system  that

enables autonomous operation.

4.4 Evidence: Specific Cases

Case A — Autonomous bug fix, external user (3 hours): Issue  #898:
an external user ( namasl ) requested GPU drill-down details on February
12. PR #900 was opened, reviewed, and merged within 3 hours — 624

additions across 7 files. The user did not need to write any code.

Case  B  —  System  identifies  user  misunderstanding  (10  minutes):

Issue  #5475:  a  user  reported  that  a  cluster  was  marked  “healthy”  while

pods  were  in  ImagePullBackOff  state. The  system  responded  within  10

minutes, explaining that cluster health reflects infrastructure health (node

readiness,  API  reachability),  which  is  architecturally  separate  from

workload health. The user had a Kubernetes knowledge gap, not a bug.

The codebase’s tests and health-check logic encoded this design decision

clearly enough for the AI to explain it.

Case  C  —  AI  failure,  PR  closed  in  8  minutes:  PR  #5289:  Copilot

generated a PR claiming to add “drag and drop for new workloads” but
the  change  was  labeled  size/XS   and  did  not  match  the  issue
requirements.  Created  and  closed  within  8  minutes.  This  is  the  system

working  correctly  —  low-quality  output  is  caught  and  rejected,  feeding

back into acceptance rate tracking.

Case  D  —  Auto-QA  catches  real  quality  gap  (11  minutes):  Issue

#5390:  the Auto-QA  system  detected  hardcoded  user-facing  strings  that

needed i18n extraction. Filed at 02:03, resolved at 02:14 — 11 minutes

from  detection  to  fix.  No  human  initiated  this;  the  system  found  the

problem and fixed it autonomously.

Case  E  —  Aggregate  failure  data  drives  self-correction:  The
operator  category accumulated 129 closed PRs against only 11 merged
(8% acceptance rate). The Auto-QA tuning system automatically set the

rotation  weight  to  0,  blocking  all  future  operator-category  issues.  CI
resources were redirected to  a11y  (62% acceptance, weight 0.93). This
is the self-tuning feedback loop in action — the system learned what the

maintainer values and stopped wasting cycles on what doesn’t work.

4.5 Quantitative Summary

Table 3: System Metrics

Metric

Value

GitHub Actions workflows

63 (22 AI-specific)

Nightly test suites

32

Code coverage

91% across 12 shards

Level

L4

L3

L3

Distinct feedback loops

33 (see Table 2)

L2-L5

Bug-to-fix time

~30 minutes

Feature-to-implementation time

~60 minutes

Triage loop frequency

Every 15 minutes, 4 repos

PR monitor frequency

Every 60 seconds

GA4 error monitoring

Hourly

Auto-QA quality checks

4x daily, 8 layers

Concurrent AI sessions

Multiple via git worktrees

Time from zero to Level 5

~82 days

L4

L4

L4

L4

L3

L4

L4

—

Table 4: Aggregate Data (Jan 16 – Apr 8, 2026, 82 days)

Metric

Total commits

Total PRs merged

Total PRs closed (unmerged)

Overall PR acceptance rate

Value

3,587

2,765

632

81.4%

Copilot PRs merged / total

319 / 812 (39.3%)

Metric

Total issues opened

Total issues closed

Value

2,115

2,109 (99.7%)

Issues with  ai-fix-requested  label

1,523 (99.93% resolved)

Auto-QA issues created and resolved

523 (95.2% completed)

Average PRs merged per day

33.7

Contributors

24 (GitHub) / 30 (git)

5. Discussion

5.1 The Intelligence Is in the System, Not the Model

The AI model — whether Claude Opus, GPT-4, or any successor — is a

commodity  component  within  this  architecture.  The  differentiation  lies

entirely  in  the  surrounding  infrastructure:  the  instruction  files,  the  test

suites,  the  feedback  loops,  the  monitoring  workflows,  the  tuning

configurations.  Switching  from  one AI  model  to  another  would  require

modest effort. Rebuilding the surrounding system would require months.

This has a practical implication: organizations investing in “AI strategy”

should  invest  primarily  in  the  infrastructure  of  intelligence  —  the  tests,

the metrics, the feedback loops — not in selecting the perfect model.

5.2 Ask Questions, Not Commands

One  operational  insight  proved  disproportionately  valuable:  prompting

with “why didn’t you catch this?” rather than “fix this bug.” The former

produces root cause analysis and systemic improvements — a new test, a

new rule in the instruction files, a pattern that prevents the entire class of

error.  The  latter  produces  a  patch.  Over  time,  questioning  compounds

into  a  self-improving  system;  commanding  produces  a  sequence  of

isolated fixes.

This  is  the  operational  manifestation  of  the  Level  1→2  transition:

questions produce instructions as a side effect.

5.3 Refactoring as Imperative

In  traditional  development,  refactoring  competes  with  feature  work  for

engineering  time.  In AI-augmented  codebases,  this  calculus  inverts. AI

agents  perform  measurably  better  on  clean,  well-structured  code.

Technical debt directly degrades AI output quality, manifesting as lower

acceptance  rates,  more  cascading  failures,  and  longer  review  cycles.

Refactoring is not a tax — it is an investment that pays returns on every

subsequent AI session.

5.4 Telemetry as the Nervous System

One  aspect  that  deserves  emphasis:  the  telemetry  layer  gives  me

immediate,  continuous  feedback  on  engagement,  reach,  and  —  most

importantly  —  failures  and  errors.  GA4
tracks  custom  events
( ksc_error   with  category,  page,  and  detail  dimensions),  NPS  surveys
capture  user  sentiment,  the  analytics  dashboard  visualizes  funnels  from

landing  to  agent  connection  to  mission  completion.  When  something

breaks in production, I don’t find out from a user report days later. The

hourly error monitor catches it, creates an issue, and an agent is already

working  on  a  fix.  When  users  disengage  from  a  page,  the  engagement

analytics surface it. This telemetry isn’t a nice-to-have bolt-on — it is the

sensory input that makes the entire feedback loop architecture functional.

Without it, the system would be autonomous but blind.

5.5 Community-Steered Open Source

At  Level  5,  something  unexpected  emerged:  the  project  became  what  I

consider the purest form of community-steered open source. Users don’t

need  to  write  code.  They  don’t  need  to  understand  Go  or  TypeScript.

They open an issue describing what they want, and the system builds it.

Bug reports are resolved in 30 minutes. Feature requests are implemented

in an hour. Twenty-four hours a day, seven days a week.

The  community  doesn’t  just  influence  the  roadmap  —  they  are  the

roadmap. Every issue is a direction. Every piece of feedback is a steering

input. This may represent a new model for open source maintenance that

addresses  the  chronic  maintainer  burnout  problem:  community-steered,

AI-implemented, human-governed.

5.6 Limitations

This paper reports on a single case study with a single maintainer and a

specific technology stack. KubeStellar Console is primarily a dashboard

(UI/API  integration),  not  safety-critical  or  algorithmically  complex

software  —  generalizability  to  other  domains  is  an  open  question.  The

solo maintainer context may not directly map to large teams, though the

model’s  levels  are  defined  by  system  properties  rather  than  team

properties. There is inherent survivorship bias: I report on a system that

reached  Level  5  and  do  not  observe  systems  that  failed  at  intermediate

levels. The maturity model has not been independently validated across

multiple organizations.

6. Implications for Practice

For individual developers: Start with Level 2. Writing a CLAUDE.md

file  takes  30  minutes  and  immediately  improves AI  output  consistency.

Then invest in test coverage — not just for correctness, but as the trust

mechanism that enables every subsequent level. Don’t overthink it. Get it

working, normalize things later.

For  engineering  leaders:  Assess  your  current  level  honestly.  Most

organizations  are  at  Level  1-2.  The  transition  to  Level  3  requires

investment  in  measurement  infrastructure  —  acceptance  rate  tracking,

coverage  gating,  error  monitoring.  Budget  for  the  “infrastructure  of

intelligence,” not just AI tool licenses.

For the open source community: Level 5 suggests a new maintenance

model: community-steered, AI-implemented, human-governed. Standard

instruction  file  formats  (CLAUDE.md,  copilot-instructions.md)  could

become  as  important  as  README.md  and  CONTRIBUTING.md. This

could address the sustainability crisis in open source maintenance.

For  researchers:  The  model  needs  multi-case  validation.  I  invite

researchers to apply ACMM to other projects and report findings. Open

questions include: Does the model apply to safety-critical systems? How

do large teams progress through levels? What are the economic inflection

points for each transition?

7. Conclusion

This  paper  presented  the  AI  Codebase  Maturity  Model  (ACMM),  a  5-

level framework for understanding how codebases evolve from basic AI-

assisted  coding  to  self-sustaining  systems.  The  model  was  validated

through  a  4-month  practitioner  experience  report  building  KubeStellar

Console from scratch with AI coding agents.

The  central  finding  is  that  systematic  investment  in  the  infrastructure

surrounding  AI  tools  —  instruction  files,  test  suites,  metrics,  feedback

loops  —  yields  compounding  returns.  Each  maturity  level  depends  on

feedback mechanisms established at the previous level. You cannot skip

levels. And at every level, the thing that unlocks the next one is another

feedback mechanism.

Testing proved to be the single most important investment: the volume of

test  cases,  the  coverage  thresholds,  and  the  reliability  of  test  execution

together  form

the

trust  foundation

that  makes  autonomous  AI

development possible.

Most developers and organizations using AI for coding are at Level 1 or

2. That is where everyone starts. The point is not to race to Level 5. The

point  is  to  know  what  the  next  level  looks  like  and  what  feedback

mechanism unlocks it.

The  code  remembers  what  I  forget.  The  tests  catch  what  I  miss.  The

feedback loops keep the system honest. But the values, the priorities, the

decision about what to build and what to skip — that’s still me.

References

[1] M. C. Paulk, B. Curtis, M. B. Chrissis, and C. V. Weber, “Capability

Maturity Model, Version 1.1,” IEEE Software, vol. 10, no. 4, pp. 18–27,

1993.

[2]  CMMI  Institute,  “CMMI  for  Development,  Version  1.3,”  Software

Engineering Institute, Carnegie Mellon University, 2010.

[3]  J.  Bach,  “The  Challenge  of  ‘Good  Enough’  Software,”  American

Programmer, vol. 8, no. 10, 1995.

[4]  N.  Forsgren,  J.  Humble,  and  G.  Kim,  Accelerate:  The  Science  of

Lean Software and DevOps. IT Revolution Press, 2018.

[5] GitHub, “GitHub Copilot: Your AI pair programmer,” 2021. [Online].

Available: https://github.com/features/copilot

[6]  Anthropic,

“Claude  Code,”

2025.

[Online].  Available:

https://docs.anthropic.com/en/docs/claude-code

[7]  C.  E.  Jimenez  et  al.,  “SWE-bench:  Can  Language  Models  Resolve

Real-World GitHub Issues?” arXiv preprint arXiv:2310.06770, 2023.

[8] S. Peng, E. Kalliamvakou, P. Cihon, and M. Demirer, “The Impact of

AI  on  Developer  Productivity:  Evidence  from  GitHub  Copilot,”  arXiv

preprint arXiv:2302.06590, 2023.

[9]  A.  Ziegler  et  al.,  “Productivity  Assessment  of  Neural  Code

Completion,”  in  Proc.  of  the  6th  ACM  SIGPLAN  International

Symposium on Machine Programming, 2022.

[10]  Google  Cloud,  “MLOps:  Continuous  delivery  and  automation

pipelines

in  machine

learning,”

2020.

[Online].  Available:

https://cloud.google.com/architecture/mlops-continuous-delivery-and-

automation-pipelines-in-machine-learning

[11]  N.  Eghbal,  Working  in  Public:  The  Making  and  Maintenance  of

Open Source Software. Stripe Press, 2020.

[12]  KubeStellar  Project,  “KubeStellar  Console,”  2025.  [Online].

Available:

https://console.kubestellar.io.

Source

code:

https://github.com/kubestellar/console

[13] W. S. Humphrey, Managing the Software Process. Addison-Wesley,

1989.

[14]  D.  Shapiro,  “The  Five  Levels:  From  Spicy  Autocomplete  to  the

Dark

Factory,”

Jan.

2026.

[Online].

Available:

https://www.danshapiro.com/blog/2026/01/the-five-levels-from-spicy-

autocomplete-to-the-software-factory/

[15]  BCG  Platinion,  “The  Dark  Software  Factory,”  2026.  [Online].

Available:

https://www.bcgplatinion.com/insights/the-dark-software-

factory

[16]  Stanford  Law  School  CodeX,  “Built  by Agents, Tested  by Agents,

Trusted

by  Whom?”

Feb.

2026.

[Online].  Available:

https://law.stanford.edu/2026/02/08/built-by-agents-tested-by-agents-

trusted-by-whom/

[17]  ELEKS,  “AI-SDLC  Maturity  Model:  Traditional  to  Autonomous

Development,” 2025. [Online]. Available: https://eleks.com/blog/ai-sdlc-

maturity-model/

[18] G. Gondim, “AI Development Maturity Model,” DEV Community,

2025.

[Online].  Available:  https://dev.to/ggondim/ai-development-

maturity-model-4i47

[19] Gigacore, “AI Maturity Model for Software Engineering Teams (AI-

MM

SET),”

GitHub,

2025.

[Online].

Available:

https://github.com/Gigacore/AI-Maturity-Model


