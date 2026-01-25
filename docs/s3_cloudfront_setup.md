# S3 and CloudFront Setup for Library Files

This guide covers setting up AWS S3 and CloudFront for serving Library files with signed URLs (restricted access). Signed URLs provide security by:

- Restricting access to authenticated users only
- Expiring URLs after a set time (default: 15 minutes)
- Preventing permanent hotlinks to files

## Overview

```
┌─────────┐     ┌────────────┐     ┌─────────────┐     ┌────────┐
│  User   │────▶│   StrataSave   │────▶│  CloudFront │────▶│   S3   │
└─────────┘     └────────────┘     └─────────────┘     └────────┘
                 Generates           Validates          Stores
                 signed URL          signature          files
```

1. User requests to view/download a file
2. StrataSave generates a signed CloudFront URL
3. User's browser requests the file from CloudFront
4. CloudFront validates the signature and serves the file from S3

## Prerequisites

- AWS account with appropriate permissions
- AWS CLI installed and configured (optional, but helpful)
- OpenSSL for generating keys

## Step 1: Create an S3 Bucket

### Via AWS Console

1. Go to **S3** in the AWS Console
2. Click **Create bucket**
3. Configure:
   - **Bucket name**: e.g., `myapp-library-files`
   - **Region**: Choose your preferred region (e.g., `us-east-1`)
   - **Block Public Access**: Keep **all options enabled** (we want private access)
   - **Versioning**: Optional, but recommended for production
4. Click **Create bucket**

### Via AWS CLI

```bash
aws s3 mb s3://myapp-library-files --region us-east-1
```

## Step 2: Create a CloudFront Origin Access Control (OAC)

This allows CloudFront to access your private S3 bucket.

### Via AWS Console

1. Go to **CloudFront** > **Origin access** > **Origin access controls**
2. Click **Create control setting**
3. Configure:
   - **Name**: e.g., `myapp-library-oac`
   - **Signing behavior**: Sign requests (recommended)
   - **Origin type**: S3
4. Click **Create**

## Step 3: Create a CloudFront Distribution

### Via AWS Console

1. Go to **CloudFront** > **Distributions**
2. Click **Create distribution**
3. Configure Origin:
   - **Origin domain**: Select your S3 bucket (`myapp-library-files.s3.us-east-1.amazonaws.com`)
   - **Origin access**: Select **Origin access control settings**
   - **Origin access control**: Select the OAC you created
4. Configure Default cache behavior:
   - **Viewer protocol policy**: Redirect HTTP to HTTPS
   - **Allowed HTTP methods**: GET, HEAD
   - **Restrict viewer access**: **Yes**
   - **Trusted authorization type**: Trusted key groups (recommended)
5. Configure Settings:
   - **Price class**: Choose based on your needs
   - **Default root object**: Leave empty
6. Click **Create distribution**

**Important**: After creating the distribution, you'll see a banner prompting you to update the S3 bucket policy. Click **Copy policy** and proceed to Step 4.

## Step 4: Update S3 Bucket Policy

Allow CloudFront to access your S3 bucket.

### Via AWS Console

1. Go to **S3** > your bucket > **Permissions** > **Bucket policy**
2. Paste the policy CloudFront provided, or use this template:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllowCloudFrontServicePrincipal",
            "Effect": "Allow",
            "Principal": {
                "Service": "cloudfront.amazonaws.com"
            },
            "Action": "s3:GetObject",
            "Resource": "arn:aws:s3:::myapp-library-files/*",
            "Condition": {
                "StringEquals": {
                    "AWS:SourceArn": "arn:aws:cloudfront::YOUR_ACCOUNT_ID:distribution/YOUR_DISTRIBUTION_ID"
                }
            }
        }
    ]
}
```

Replace:
- `myapp-library-files` with your bucket name
- `YOUR_ACCOUNT_ID` with your AWS account ID
- `YOUR_DISTRIBUTION_ID` with your CloudFront distribution ID

## Step 5: Create a CloudFront Key Pair

CloudFront uses RSA key pairs to sign URLs. You'll create a private key (kept secret) and a public key (uploaded to AWS).

### Generate the Key Pair

```bash
# Generate a 2048-bit RSA private key
openssl genrsa -out cloudfront-private-key.pem 2048

# Extract the public key
openssl rsa -pubout -in cloudfront-private-key.pem -out cloudfront-public-key.pem
```

**Important**: Keep `cloudfront-private-key.pem` secure. This file will be used by StrataSave to sign URLs.

### Upload the Public Key to CloudFront

1. Go to **CloudFront** > **Public keys**
2. Click **Create public key**
3. Configure:
   - **Name**: e.g., `myapp-library-signing-key`
   - **Key**: Paste the contents of `cloudfront-public-key.pem`
4. Click **Create public key**
5. **Copy the Key ID** (e.g., `K2XXXXXXXXXX`) - you'll need this for StrataSave configuration

### Create a Key Group

1. Go to **CloudFront** > **Key groups**
2. Click **Create key group**
3. Configure:
   - **Name**: e.g., `myapp-library-key-group`
   - **Public keys**: Select the public key you just created
4. Click **Create key group**

### Associate Key Group with Distribution

1. Go to **CloudFront** > **Distributions** > your distribution
2. Click the **Behaviors** tab
3. Select the default behavior and click **Edit**
4. Under **Restrict viewer access**:
   - Ensure **Yes** is selected
   - **Trusted authorization type**: Trusted key groups
   - **Trusted key groups**: Select your key group
5. Click **Save changes**

## Step 6: Configure StrataSave

Set these environment variables for your StrataSave installation:

```bash
# Storage type
STRATASAVE_STORAGE_TYPE=s3

# S3 configuration
STRATASAVE_STORAGE_S3_REGION=us-east-1
STRATASAVE_STORAGE_S3_BUCKET=myapp-library-files
STRATASAVE_STORAGE_S3_PREFIX=library/

# CloudFront configuration (signed URLs)
STRATASAVE_STORAGE_CF_URL=https://d1234567890.cloudfront.net
STRATASAVE_STORAGE_CF_KEYPAIR_ID=K2XXXXXXXXXX
STRATASAVE_STORAGE_CF_KEY_PATH=/path/to/cloudfront-private-key.pem
```

Replace:
- `us-east-1` with your bucket's region
- `myapp-library-files` with your bucket name
- `library/` with your preferred prefix (keeps files organized)
- `d1234567890.cloudfront.net` with your CloudFront distribution domain
- `K2XXXXXXXXXX` with your public key ID
- `/path/to/cloudfront-private-key.pem` with the path to your private key file

### AWS Credentials

StrataSave needs AWS credentials to upload files to S3. Use one of these methods:

**Option A: Environment variables**
```bash
AWS_ACCESS_KEY_ID=AKIAXXXXXXXXXXXXXXXX
AWS_SECRET_ACCESS_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**Option B: IAM Role (recommended for EC2/ECS)**
Attach an IAM role with S3 write permissions to your instance.

**Option C: AWS credentials file**
Configure `~/.aws/credentials` on the server.

### Required IAM Permissions

The AWS credentials need these S3 permissions on your bucket:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:GetObject",
                "s3:DeleteObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::myapp-library-files",
                "arn:aws:s3:::myapp-library-files/*"
            ]
        }
    ]
}
```

## Step 7: Test the Configuration

1. Start StrataSave with the new configuration
2. Log in as an admin
3. Go to **Library** and upload a test file
4. Click **View** or **Download** on the file
5. Verify the URL contains CloudFront domain and signature parameters

A signed URL looks like:
```
https://d1234567890.cloudfront.net/library/2026/01/abc123-document.pdf?Expires=1737312000&Signature=...&Key-Pair-Id=K2XXXXXXXXXX
```

## Troubleshooting

### "Access Denied" when viewing files

- Verify the S3 bucket policy allows CloudFront access
- Check that the CloudFront distribution's origin is configured correctly
- Ensure the OAC is attached to the origin

### "Invalid signature" errors

- Verify `STRATASAVE_STORAGE_CF_KEYPAIR_ID` matches the public key ID in CloudFront
- Ensure the private key file is readable by StrataSave
- Check that the public key in CloudFront matches the private key

### Files upload but can't be viewed

- Wait a few minutes for CloudFront to propagate (new distributions take 5-10 minutes)
- Check CloudFront distribution status is "Deployed"
- Verify the S3 prefix in StrataSave matches your setup

### "SignatureDoesNotMatch" from S3

- This usually means CloudFront is bypassed; check the URL domain
- Verify the distribution's origin points to the correct bucket

## Security Best Practices

1. **Protect the private key**: Store it securely, never commit to version control
2. **Use IAM roles** instead of access keys when possible
3. **Rotate keys periodically**: Create new key pairs and update configuration
4. **Enable CloudFront logging**: Monitor access patterns for suspicious activity
5. **Set appropriate TTL**: The default 15-minute URL expiry balances security and usability

## Alternative: Public CloudFront (No Signing)

If your files don't require restricted access, you can use CloudFront without signing:

```bash
STRATASAVE_STORAGE_TYPE=s3
STRATASAVE_STORAGE_S3_REGION=us-east-1
STRATASAVE_STORAGE_S3_BUCKET=myapp-library-files
STRATASAVE_STORAGE_S3_PREFIX=library/
STRATASAVE_STORAGE_CF_URL=https://d1234567890.cloudfront.net
# Don't set CF_KEYPAIR_ID or CF_KEY_PATH
```

For this to work, configure CloudFront without "Restrict viewer access" and ensure S3 allows CloudFront to read objects.
