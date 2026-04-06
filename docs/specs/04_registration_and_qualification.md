# 04. Registration and Qualification

## Participation Requirements
A node must satisfy at least the following to participate:

- it is operated on Symbol mainnet;
- it has a voting key valid for the current epoch;
- it exposes the monitored endpoint required by the program;
- it runs Program Agent;
- it accepts the program terms.

## Registration Inputs
- operator address
- node identification data
- voting-key validation data
- endpoint information
- alert contact
- signed registration challenge
- Program Agent linkage data

## Qualification Philosophy
The priority is not "strong hardware on paper."
The priority is "currently able to continue voting safely and observably."

## Observation Window
Minimum 72 hours before a newly registered node can become Qualified.

## Qualified Node Criteria
A node is Qualified only if all of the following are satisfied:
- daily availability >= 99.0%;
- finalized lag within threshold on >= 95% of valid probes;
- chain synchronization within threshold on >= 95% of valid probes;
- valid voting key for the relevant current epoch;
- Program Agent heartbeat is current;
- no critical anomalies or confirmed abuse.

## Verification Notes
- voting-key validity should be checked against current-epoch conditions;
- registered voting keys should be verifiable from account-linked key data;
- qualification is separate from reward eligibility.

## Important Separation
A node can be:
- registered but not yet observed;
- observed but not Qualified;
- Qualified but not reward-eligible due to anti-concentration filtering.
