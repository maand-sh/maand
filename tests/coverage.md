# test coverage

## worker.json
 1. build without worker.json
 2. build with empty worker.json
 3. build with '[]' worker.json - pass
 4. build without 'host' attribute in worker.json - fail
 5. build with invalid worker.json
 6. build with duplicate host ip - fail
 7. build with labels as int - fail
 8. build with labels and defaults - pass
 9. build with duplicate labels - fail
10. build worker labels added later - pass
11. build worker labels removed later - pass
12. build worker labels updated later - pass
13. build worker added later - pass
14. build worker updated later - pass
15. build worker removed later - pass
16. build worker replaced later - pass
17. build worker tags as int - fail
18. build worker tags as array - fail
19. build worker tags as object - pass
20. build worker tags added later - pass
21. build worker tags removed later - pass
22. build worker tags updated later - pass
23. build worker with memory added - pass
24. build worker with memory updated - pass
25. build worker with memory removed - pass
26. build worker with cpu added - pass
28. build worker with cpu updated - pass
29. build worker with cpu removed - pass
30. build worker cpu and memory conversion - pass
31. later remove worker.json file 

## bucket
 1. build without initialize - fail
 2. build initialize - pass
 3. build reinitialize, upgrade flow - pass
 4. run_command with args, validate logs
 5. run_command with command.sh, validate logs 
 6. initialize with existing worker.json
 7. initialize with existing files without maand.db
 8. valid bucket.conf
 9. valid bucket.jobs.conf
10. valid bucket.jobs.%.conf
11. build with new ca 
12. build and cat 
13. encrypted secrets
14. worker with multiple bucket support
15. gc removes removed allocations 
16. gc removes outdated kv entries 
17. run_command with concurrency 
18. run_command with selector
19. run_command with worker
20. with disabled.json
21. docker command required 
22. any failures returns error code
23. maand.db copied to first there nodes 

## job
 1. build job without manifest.json - fail
 2. build job with invalid manifest.json - fail
 3. build job without makefile - fail
 4. build job with 'worker' selector - pass 
 5. build job without any selector, disabled - pass
 6. build job with selector
 7. build job with selector added later
 8. build job with selector updated later 
 9. build job with selector deleted later
10. build job with worker labels added later
11. build job with worker labels updated later
12. build job with worker labels deleted later
13. build job with memory (min, max) added later 
14. build job with memory (min, max) updated later
15. build job with memory (min, max) removed later 
16. build job with memory limit exceeds on worker (bucket.jobs.conf and defaults)
17. build job with cpu (min, max) added later
18. build job with cpu (min, max) updated later 
19. build job with cpu (min, max) removed later
20. build job with cpu limit exceeds on worker (bucket.jobs.conf and defaults)
21. build job with port
22. build job with duplicate port - (same, different job)
23. build job with values on bucket.jobs.conf
24. build job with cert config, updated only on when required
25. build job with cert added later 
26. build job with cert updated later 
27. build job with cert removed later
28. build job with job command added later 
29. build job with job command updated later 
30. build job with job command removed later 
31. build jobs with deployment sequence
32. build jobs with circular references
33. build job with valid job command triggers (post_build, health_check, pre_deploy, post_deploy)
34. build job with post_build failure, should not affects build
35. build jobs with job_command with pass 'config'
36. job_command with available env values
37. build job with template and defaults variables, methods
38. build job with data, logs, bin folder - fails 
39. gc removes data, logs, bin folders from worker on removed allocation
40. build job with version 
41. build job with version added later
42. build job with version updated later
43. build job with version removed later 
44. build job with major version upgrade
45. build job with health_check
46. build job with health_check failures
47. later remove all jobs

## deployment
 1. deploy without job
 2. deploy with a job
 3. deploy update job, triggers restart all allocations 
 4. deploy update a allocation, triggers restarts a allocation
 5. deploy remove a allocation
 6. deploy disable a allocation
 7. deploy disable job
 8. deploy 3 jobs without sequence
 9. deploy 3 jobs with sequence
10. deploy 3 jobs with sequence added later
11. deploy 3 jobs with sequence updated later
12. deploy 3 jobs with sequence removed later 
13. deploy 3 jobs with sequence with all 3 updated 
14. deploy 3 jobs with sequence with all 3 updated and failed on 2nd one, 3 is not updated
15. deploy job with 1 allocation, 2 added later - test
16. deploy job with 2 allocation, 1 added and 1 removed - test deployment sequence - removed, added and updated allocations 
17. deploy job with job command, job command failures within job, hash not updated.
18. job command kv updated always committed regardless of job command failures
19. deploy 3 jobs health_check failure, stop deployment 
20. deploy job and test rolling upgrade

