// Unraid WebGUI Share Management via JavaScript
// Run these in the browser via mcp__claude-in-chrome__javascript_tool
// on a tab that is already authenticated to the Unraid WebGUI.
//
// PREREQUISITE: Tab must be on the correct page before running.
// Use mcp__claude-in-chrome__navigate to get there first.

// ============================================================
// 1. CREATE A NEW SHARE
// ============================================================
// Navigate to: https://192.168.1.50/Shares/Share?name=
// Then run:

function createUnraidShare({
  name,
  comment = '',
  pool = '',           // '' = Array, 'nvmecache' = NVMe pool
  useCache = 'no',     // 'no' = array only, 'yes' = cache+array, 'only' = pool only, 'prefer' = prefer pool
  allocator = 'highwater',
  cow = 'auto',
  floor = '0'
}) {
  const form = document.querySelector('form[action*="update"]');
  if (!form) throw new Error('Not on share creation page. Navigate to /Shares/Share?name= first');

  // Set fields
  form.querySelector('[name="shareName"]').value = name;
  form.querySelector('[name="shareComment"]').value = comment;
  form.querySelector('[name="shareFloor"]').value = floor;
  form.querySelector('[name="shareUseCache"]').value = useCache;
  form.querySelector('[name="shareNameOrig"]').value = '';

  // Set pool and trigger dynamic form update
  const poolSelect = form.querySelector('[name="shareCachePool"]');
  poolSelect.value = pool;
  poolSelect.dispatchEvent(new Event('change', {bubbles: true}));

  // Set COW
  const cowSelect = form.querySelector('[name="shareCOW"]');
  if (cowSelect) cowSelect.value = cow;

  // Submit
  form.querySelector('[name="cmdEditShare"]').click();
}

// ============================================================
// 2. SET SMB EXPORT + SECURITY (after share is created)
// ============================================================
// Navigate to: https://192.168.1.50/Shares/Share?name=<ShareName>
// Then run:

function setUnraidShareSMB({
  smbExport = 'e',        // '-' = No, 'e' = Yes, 'eh' = Yes (hidden), 'et' = Yes/Time Machine
  security = 'private',   // 'public', 'secure', 'private'
  caseSensitive = 'auto'  // 'auto', 'yes', 'forced' (force lower)
}) {
  const forms = document.querySelectorAll('form[action*="update"]');
  // SMB form is the 3rd update.htm form (index 2 among update.htm forms, index 3 overall)
  const smbForm = Array.from(forms).find(f =>
    f.querySelector('[name="shareExport"]') && f.querySelector('[name="shareSecurity"]')
  );
  if (!smbForm) throw new Error('Not on share edit page. Navigate to /Shares/Share?name=<name> first');

  smbForm.querySelector('[name="shareExport"]').value = smbExport;
  smbForm.querySelector('[name="shareSecurity"]').value = security;
  const cs = smbForm.querySelector('[name="shareCaseSensitive"]');
  if (cs) cs.value = caseSensitive;

  smbForm.querySelector('[name="changeShareSecurity"]').click();
}

// ============================================================
// 3. DELETE A SHARE
// ============================================================
// Navigate to: https://192.168.1.50/Shares/Share?name=<ShareName>
// Then run:

function deleteUnraidShare() {
  const form = document.querySelector('form[action*="update"]');
  if (!form) throw new Error('Not on share edit page');
  const nameOrig = form.querySelector('[name="shareNameOrig"]').value;
  if (!nameOrig) throw new Error('No share loaded');

  // Clear the name and submit (Unraid treats this as delete when confirmDelete is checked)
  form.querySelector('[name="shareName"]').value = '';
  const confirm = form.querySelector('[name="confirmDelete"]');
  if (confirm) confirm.checked = true;
  form.querySelector('[name="cmdEditShare"]').click();
  return 'Deleting: ' + nameOrig;
}

// ============================================================
// QUICK REFERENCE: Common share presets
// ============================================================
//
// NVMe-only share (fast, no mover):
//   createUnraidShare({ name: 'X', comment: '...', pool: 'nvmecache', useCache: 'only' })
//
// Array-only share (bulk storage):
//   createUnraidShare({ name: 'X', comment: '...', pool: '', useCache: 'no' })
//
// Cache-first share (NVMe write, mover sends to array):
//   createUnraidShare({ name: 'X', comment: '...', pool: 'nvmecache', useCache: 'yes' })
//
// After creation, set SMB:
//   setUnraidShareSMB({ smbExport: 'e', security: 'private' })

// ============================================================
// FORM FIELD REFERENCE (captured 2026-03-28)
// ============================================================
//
// Form 1 - Share Settings (/update.htm):
//   shareNameOrig    (hidden) - original name, empty for new
//   shareUseCache    (hidden) - 'no'|'yes'|'only'|'prefer'
//   shareCachePool   (select) - ''=Array, 'nvmecache'=NVMe
//   shareCachePool2  (hidden) - secondary pool
//   shareName        (text)   - share name
//   shareComment     (text)   - description
//   shareFloor       (text)   - minimum free space
//   shareCOW         (select) - 'auto'|'no'
//   shareAllocator   (hidden) - set by pool selection
//   shareAllocator1  (select) - 'highwater'|'fillup'|'mostfree'
//   shareSplitLevel  (hidden)
//   shareSplitLevel1 (select)
//   shareInclude     (hidden)
//   shareExclude     (hidden)
//   confirmDelete    (checkbox)
//   cmdEditShare     (submit) - 'Apply' or 'Add Share'
//   csrf_token       (hidden)
//
// Form 3 - SMB Security (/update.htm):
//   shareName            (hidden)
//   shareExport          (select) - '-'|'e'|'eh'|'et'|'eth'
//   shareVolsizelimit    (text)
//   shareCaseSensitive   (select) - 'auto'|'yes'|'forced'
//   shareSecurity        (select) - 'public'|'secure'|'private'
//   changeShareSecurity  (submit)
//   csrf_token           (hidden)
//
// Form 2 - NFS Security (/update.htm):
//   shareName            (hidden)
//   shareExportNFS       (select) - '-'|'e'
//   shareSecurityNFS     (select) - 'public'|'secure'|'private'
//   changeShareSecurityNFS (submit)
//   csrf_token           (hidden)
