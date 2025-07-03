## S3 Migration Guide

1. Create Hetzner S3 bucket
2. Transfer data from MinIO to S3
3. Update Milvus configuration to use S3
4. Validate data integrity

### Verification Steps
- Check object counts in both storage systems
- Run integration tests
- Monitor system performance