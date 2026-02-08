# Google Cloud Setup for Android Management API

**Purpose**: Enable Android device management in Sprint 2  
**Cost**: Free (free tier sufficient for development)  
**Time**: 30-45 minutes  
**Required for**: S2-05 (Android enrollment)

---

## Prerequisites

- Google account (personal or work email)
- Credit card (for verification only, won't be charged on free tier)
- Browser

---

## Step-by-Step Setup

### Step 1: Create Google Cloud Project (5 minutes)

1. **Go to Google Cloud Console**
   - Visit: https://console.cloud.google.com
   - Sign in with your Google account

2. **Accept Terms of Service**
   - First-time users: Accept Google Cloud Terms of Service
   - Set up billing (required even for free tier)
   - Add credit card (won't be charged unless you upgrade)

3. **Create New Project**
   - Click "Select a project" dropdown (top bar)
   - Click "New Project"
   - **Project name**: `local-mdm-dev` (or your choice)
   - **Organization**: Leave as "No organization" (unless you have one)
   - Click "Create"
   - Wait 30 seconds for project creation

4. **Verify Project Selected**
   - Ensure "local-mdm-dev" is selected in top bar
   - Note the **Project ID** (e.g., `local-mdm-dev-123456`)
   - You'll need this later

---

### Step 2: Enable Android Management API (2 minutes)

1. **Open API Library**
   - In left sidebar: Click "APIs & Services" → "Library"
   - Or visit: https://console.cloud.google.com/apis/library

2. **Search for Android Management API**
   - Search box: Type "Android Management API"
   - Click "Android Management API" from results

3. **Enable API**
   - Click "Enable" button
   - Wait 10-20 seconds for activation
   - You'll see "API enabled" confirmation

4. **Verify Enabled**
   - Go to "APIs & Services" → "Enabled APIs & services"
   - Confirm "Android Management API" is listed

---

### Step 3: Create Service Account (10 minutes)

1. **Open IAM & Admin**
   - Left sidebar: "IAM & Admin" → "Service Accounts"
   - Or visit: https://console.cloud.google.com/iam-admin/serviceaccounts

2. **Create Service Account**
   - Click "+ CREATE SERVICE ACCOUNT" (top)
   
3. **Service Account Details**
   - **Service account name**: `local-mdm-android`
   - **Service account ID**: `local-mdm-android` (auto-filled)
   - **Description**: `Service account for Local MDM Android device management`
   - Click "CREATE AND CONTINUE"

4. **Grant Permissions**
   - **Select a role**: Click dropdown
   - Search for: `Android Management User`
   - Select: "Android Management User"
   - Click "CONTINUE"

5. **Grant Users Access** (Optional)
   - Skip this step (click "DONE")
   - You don't need to grant other users access

6. **Verify Service Account Created**
   - You should see `local-mdm-android@local-mdm-dev-123456.iam.gserviceaccount.com`

---

### Step 4: Create and Download JSON Key (5 minutes)

1. **Open Service Account**
   - Click on the service account you just created
   - (`local-mdm-android@...`)

2. **Go to Keys Tab**
   - Click "KEYS" tab (top)
   - Click "ADD KEY" dropdown
   - Select "Create new key"

3. **Create Key**
   - **Key type**: Select "JSON" (default)
   - Click "CREATE"

4. **Download Key File**
   - JSON key file automatically downloads
   - **Filename**: `local-mdm-dev-123456-abc123def456.json`
   - **IMPORTANT**: This file contains credentials - keep it secure!

5. **Save Key File Securely**
   ```bash
   # Recommended location (on your machine):
   mkdir -p ~/local-mdm-secrets
   mv ~/Downloads/local-mdm-dev-*.json ~/local-mdm-secrets/google-service-account.json
   chmod 600 ~/local-mdm-secrets/google-service-account.json
   ```

---

### Step 5: Note Required Information (2 minutes)

You'll need to provide these to the development environment:

1. **Project ID**
   - Found in: Google Cloud Console top bar
   - Example: `local-mdm-dev-123456`

2. **Service Account Email**
   - Found in: IAM & Admin → Service Accounts
   - Example: `local-mdm-android@local-mdm-dev-123456.iam.gserviceaccount.com`

3. **JSON Key File Path**
   - Where you saved the file
   - Example: `~/local-mdm-secrets/google-service-account.json`

---

## Providing Credentials to Development

### Option 1: Environment Variables (Recommended)

```bash
# Add to your shell profile (~/.zshrc or ~/.bashrc)
export GOOGLE_APPLICATION_CREDENTIALS="$HOME/local-mdm-secrets/google-service-account.json"
export GOOGLE_CLOUD_PROJECT="local-mdm-dev-123456"
```

Then reload:
```bash
source ~/.zshrc  # or ~/.bashrc
```

### Option 2: Configuration File

Add to `configs/config.yaml`:
```yaml
android:
  google_cloud:
    project_id: "local-mdm-dev-123456"
    credentials_file: "/Users/yourname/local-mdm-secrets/google-service-account.json"
```

### Option 3: Direct File Placement

```bash
# Copy to project secrets directory
cp ~/local-mdm-secrets/google-service-account.json \
   /path/to/local-mdm/secrets/google-service-account.json
```

---

## Verification Steps

### Test 1: Verify API Access (Using gcloud CLI)

```bash
# Install gcloud CLI (if not installed)
# macOS: brew install google-cloud-sdk

# Authenticate with service account
gcloud auth activate-service-account \
  --key-file=~/local-mdm-secrets/google-service-account.json

# Set project
gcloud config set project local-mdm-dev-123456

# Test API access
gcloud services list --enabled | grep androidmanagement
```

**Expected output**: `androidmanagement.googleapis.com`

### Test 2: Verify Credentials in Code

Once Sprint 2 code is ready, we'll add a test endpoint:

```bash
# Test Android API connectivity
curl http://localhost:8080/api/v1/android/test-connection
```

**Expected response**:
```json
{
  "status": "connected",
  "project_id": "local-mdm-dev-123456",
  "api_enabled": true
}
```

---

## Security Best Practices

### ✅ DO:
- Store JSON key file outside of git repository
- Use environment variables for credentials
- Set file permissions to 600 (owner read/write only)
- Add `secrets/` directory to `.gitignore`
- Rotate keys periodically (every 90 days)

### ❌ DON'T:
- Commit JSON key file to git
- Share key file via email or chat
- Store in public locations
- Use production keys for development

---

## Troubleshooting

### Issue: "API not enabled"
**Solution**: 
1. Go to APIs & Services → Library
2. Search "Android Management API"
3. Click "Enable"

### Issue: "Permission denied"
**Solution**:
1. Go to IAM & Admin → Service Accounts
2. Click on service account
3. Click "PERMISSIONS" tab
4. Verify "Android Management User" role is assigned

### Issue: "Invalid credentials"
**Solution**:
1. Re-download JSON key file
2. Verify file path is correct
3. Check file permissions (should be 600)
4. Ensure GOOGLE_APPLICATION_CREDENTIALS points to correct file

### Issue: "Quota exceeded"
**Solution**:
- Free tier limits: 10,000 API calls/day
- For development, this is more than sufficient
- If exceeded, wait 24 hours or upgrade to paid tier

---

## Cost Information

### Free Tier (Sufficient for Sprint 2)
- **API Calls**: 10,000/day (free)
- **Device Management**: First 10,000 devices (free)
- **Storage**: 5 GB (free)

### Typical Sprint 2 Usage
- **API Calls**: ~100-500/day (well within free tier)
- **Devices**: 1-10 test devices
- **Cost**: $0

### When You Might Pay
- More than 10,000 API calls/day
- More than 10,000 managed devices
- Using other Google Cloud services

**For Sprint 2 development**: You will stay in free tier.

---

## What to Provide for Sprint 2

### Minimal (Required)

Send me the **JSON key file** via secure method:

**Option A**: Place in project directory
```bash
# On your machine:
cp ~/local-mdm-secrets/google-service-account.json \
   /path/to/local-mdm/secrets/google-service-account.json
```

**Option B**: Provide file path
```
File location: /Users/yourname/local-mdm-secrets/google-service-account.json
```

### Additional (Helpful)

Provide these values:
```
Project ID: local-mdm-dev-123456
Service Account Email: local-mdm-android@local-mdm-dev-123456.iam.gserviceaccount.com
```

---

## Timeline

| Step | Time | Can Skip? |
|------|------|-----------|
| Create Google Cloud account | 5 min | No |
| Create project | 5 min | No |
| Enable Android Management API | 2 min | No |
| Create service account | 10 min | No |
| Download JSON key | 5 min | No |
| Configure credentials | 5 min | No |
| **Total** | **30-45 min** | - |

---

## When to Do This

### Option 1: Before Sprint 2 (Recommended)
- Do setup now (30-45 min)
- Verify credentials work
- No blockers when starting Sprint 2

### Option 2: During Sprint 2
- Start Sprint 2 with mocks
- Set up Google Cloud when ready for Android work (S2-05)
- S2-01 (macOS) and S2-03 (Windows) don't need it

### Option 3: Skip for Now
- Develop S2-05 with mocked API responses
- Set up Google Cloud later for real device testing (F-01)

---

## Recommendation

✅ **Set up now** (30-45 minutes)

**Why**:
- Free and quick
- Removes blocker for S2-05
- Can test Android API during development
- No cost or risk

**Alternative**:
- Start Sprint 2 with S2-01 (macOS) or S2-03 (Windows)
- Set up Google Cloud when you reach S2-05

---

## Summary Checklist

- [ ] Create Google Cloud account
- [ ] Create project (`local-mdm-dev`)
- [ ] Enable Android Management API
- [ ] Create service account (`local-mdm-android`)
- [ ] Assign "Android Management User" role
- [ ] Create and download JSON key file
- [ ] Save key file securely
- [ ] Note Project ID
- [ ] Note Service Account Email
- [ ] Provide credentials to development environment

**Estimated Time**: 30-45 minutes  
**Cost**: $0 (free tier)  
**Difficulty**: Easy (follow screenshots in Google Cloud Console)

---

## Need Help?

If you get stuck:
1. Check "Troubleshooting" section above
2. Google Cloud has excellent documentation: https://cloud.google.com/android-management/docs
3. Let me know which step you're on and what error you see

---

**Ready to set up?** Follow the steps above and let me know when you have the JSON key file ready!
