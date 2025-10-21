const fs = require('fs');
const path = require('path');
const axios = require('axios');
const FormData = require('form-data');

const {
  BASE_URL,    // e.g. http://localhost:8080
  CLUSTER,     // e.g. demo
  TOKEN,       // bearer token
  NAMESPACE,   // e.g. default
  POD,         // e.g. nginx-abc123
  CONTAINER,   // e.g. nginx
  TARGET_DIR,  // e.g. /tmp
  LOCAL_DIR    // local dir path
} = process.env;

function requireEnv(name) {
  if (!process.env[name]) {
    console.error(`Missing env var: ${name}`);
    process.exit(2);
  }
}

['BASE_URL','CLUSTER','TOKEN','NAMESPACE','POD','CONTAINER','TARGET_DIR','LOCAL_DIR'].forEach(requireEnv);

const endpoint = `${BASE_URL.replace(/\/$/, '')}/k8s/cluster/${CLUSTER}/file/upload`;

async function uploadOne(filePath) {
  const fileName = path.basename(filePath);
  const podPath = `${TARGET_DIR.replace(/\/$/, '')}/${fileName}`;

  const form = new FormData();
  form.append('containerName', CONTAINER);
  form.append('namespace', NAMESPACE);
  form.append('podName', POD);
  form.append('path', podPath);
  form.append('fileName', fileName);
  form.append('file', fs.createReadStream(filePath));

  try {
    const resp = await axios.post(endpoint, form, {
      headers: {
        Authorization: `Bearer ${TOKEN}`,
        ...form.getHeaders()
      },
      maxContentLength: Infinity,
      maxBodyLength: Infinity
    });
    const status = resp?.data?.data?.file?.status;
    if (status === 'done') {
      return { ok: true, file: fileName };
    }
    const err = resp?.data?.data?.file?.error || 'unknown error';
    return { ok: false, file: fileName, error: err };
  } catch (e) {
    return { ok: false, file: fileName, error: e?.message || 'request failed' };
  }
}

(async () => {
  const entries = await fs.promises.readdir(LOCAL_DIR, { withFileTypes: true });
  const files = entries.filter(e => e.isFile()).map(e => path.join(LOCAL_DIR, e.name));

  if (files.length === 0) {
    console.log('No files to upload.');
    process.exit(0);
  }

  console.log(`Uploading ${files.length} file(s) to ${TARGET_DIR} ...`);

  let success = 0;
  let failed = 0;
  const failures = [];
  for (const f of files) {
    const res = await uploadOne(f);
    if (res.ok) {
      console.log(`OK: ${res.file}`);
      success++;
    } else {
      console.log(`FAIL: ${res.file} - ${res.error}`);
      failed++;
      failures.push(res.file);
    }
  }

  console.log(`\nSummary: success=${success} failed=${failed}`);
  if (failed > 0) {
    console.log(`Failed files: ${failures.join(', ')}`);
    process.exit(1);
  }
})();


