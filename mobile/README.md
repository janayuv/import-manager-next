# Import Tracker Mobile (OnePlus 11 5G Edition)

A high-performance Android companion application designed specifically for import managers and logistics leads. Built for the **OnePlus 11 5G** (OxygenOS / Android 13 & 14) with a battery-saving OLED deep-dark Material 3 theme and smooth 120Hz high-refresh rate scrolling.

---

## 🚀 Key Features (View-Only Mobile Companion)

### 1. ☁️ Google Drive Backup & Live Restore
- **Sync Live Data from Desktop**: Seamlessly update your mobile database with the latest entries from the desktop Import Manager.
- **AES-256-GCM Decryption (IMBK2)**: Direct in-app hardware-accelerated decryption matching the desktop security format (`IMBK2` / `IMBK1` PBKDF2 with 600,000 iterations).
- **Two Sync Modes**:
  1. **Direct Google Drive Cloud Sync**: Connect with your Google Drive access token to automatically fetch the latest `.imbk2` snapshot from the `ImportManagerBackups` folder.
  2. **Native File Picker**: Pick any `.imbk2`, `.db`, or `.sqlite` file directly from Google Drive app, Downloads, or local storage.
- **Offline First**: Restored data is loaded into the device's high-speed local SQLite database so you can look up shipments and duties anywhere, even without internet.

### 2. 🚢 Upcoming Shipments View
- **Active Filter Tabs**: Instantly filter by `All Active`, `In-Transit`, `Customs Clearance`, `Ready Delivery`, and `Docs Received`.
- **KPI Metrics**: Real-time counter cards showing active, in-transit, and customs-stage counts.
- **Comprehensive Cards**: View Invoice Number, Supplier, Vessel Name, Container Number, BL/AWB details, Commercial Value, and delivery countdowns.
- **Detail Sheet**: Tap any card to open a full modal bottom sheet with complete shipment parameters.

### 3. ⏱️ Shipments ETD - ETA Timeline Tracker
- **Visual Voyage Timeline**: Dynamic progress bar calculating voyage completion based on elapsed days between ETD and ETA.
- **Smart Status Badges**:
  - `In X Days` (Emerald)
  - `Arriving Today` (Amber alert)
  - `Delayed X Days` (Crimson alert if ETA has passed and delivery is pending)
- **Time Horizon Filters**: One-tap quick filtering for `Next 7 Days`, `Next 15 Days`, and `Delayed`.

### 4. 🏷️ Parts HS Code & Duty Structure View
- **Instant Search**: Search across thousands of parts by Part Number, HSN/HS Code, Description, or Supplier.
- **Customs Duty Grid**:
  - **BCD**: Basic Customs Duty % (e.g., 7.5%, 10%)
  - **SWS**: Social Welfare Surcharge % (10% on BCD)
  - **IGST**: Integrated Goods and Services Tax % (e.g., 18%)
  - **Effective Duty Rate %**: Calculated live ($Effective = BCD + SWS + IGST \times (1 + BCD + SWS)$)
- **Live Duty Calculator**: Tap any part to view an interactive simulation calculating customs duty and total landed factors on a base of ₹10,000.

### 5. 🔍 Invoice Search & Line Items
- **Instant Search**: Substring filtering by Invoice Number, Supplier, or Date.
- **Status Badges**: Filter between `Finalized` and `Draft` commercial invoices.
- **Detailed Line Items**: Tap any invoice to see every line item, quantity, unit price, customs duties, and line totals.

### 6. 📊 Landed Duty Cost (LDC) View
- **Executive KPI Dashboard**:
  - Total Landed Cost (Excl. IGST)
  - Total Assessable Value
  - Total Customs Duties (BCD + SWS)
  - Total Logistics & Port Expenses
  - Total IGST & BOE Interest
- **LDC / Qty Badges**: Prominent green badge showing exact Landed Duty Cost per piece/unit for each imported part.

---

## 📱 OnePlus 11 5G Hardware Tuning
- **Screen Resolution**: Scaled for the 1440 × 3216 20:9 Fluid AMOLED screen.
- **120Hz LTPO3 Refresh Rate**: Butter-smooth list animations and gestures.
- **True AMOLED Black Mode**: Background `#0A0F1D` and `#111827` to minimize power consumption.
- **Edge-to-Edge Navigation**: Transparent status and gesture bars.

---

## 🛠️ How to Install on OnePlus 11 5G

### Option A: USB Installation via ADB
1. On your OnePlus 11, enable **Developer Options** (tap *Build Number* 7 times in Settings > About Device).
2. Enable **USB Debugging**.
3. Connect your OnePlus 11 to your PC via USB.
4. Run:
   ```powershell
   adb install -r "d:\Import System\mobile\build\app\outputs\flutter-apk\app-debug.apk"
   ```

### Option B: Direct File Install (APK)
1. Copy `app-debug.apk` to Google Drive or send it via WhatsApp to your phone.
2. Tap the `.apk` file on your OnePlus 11 to install.
3. Grant "Install Unknown Apps" permission if prompted.
