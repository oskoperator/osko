# Labels & Annotations

OpenSLO is focused on SLOs and leaves a lot of freedom to additional information, such as ownership,
system context etc. Following is how this operator will try and get some information that is (in our opinion)
important when dealing with SLOs in an organization.

All of this information should be outputted with the metrics, in order to work with them later.

## Source of the fields

The operator should follow the same path for every field.

1. `metadata.annotations["osko/<fieldname>"]`
2. `metadata.labels.<fieldname>` 
3. Namespace `metadata.annotations["osko/<fieldname>"]` (recommended approach)
4. If none are found, default to `Unknown`

## Known Fields

### `owner`

SLO should be owned by a (single) team. 

Exposing this information as part of the output allows users to have a a good overview from the responsibility point of view.

### `system`

Most services are usually part of a bigger system.

Exposing system allows to take a look at the overall system. For example if we have SLOs for our Payment API service and then
for our post-processing Invoice jobs, we can group them under `Billing` system and aggregate the overall system functionality
per each service.

### `domain`

In bigger organizations, systems are often parts of domains. To continue the example from `system`, the `Billing` system could be part
of the `Finance` domain. 
