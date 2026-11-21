import * as React from 'react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Building2, Landmark, MapPin, Save, ShieldCheck } from 'lucide-react';
import { useCompany } from '@/contexts/CompanyContext';
import type { UpdateCompanyInput } from '@/types/company';

interface CompanyProfileModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CompanyProfileModal({ open, onOpenChange }: CompanyProfileModalProps) {
  const { currentCompany, updateCompany } = useCompany();
  const [submitting, setSubmitting] = React.useState(false);

  const [name, setName] = React.useState('');
  const [shortCode, setShortCode] = React.useState('');
  const [baseCurrency, setBaseCurrency] = React.useState('INR');
  const [iecCode, setIecCode] = React.useState('');
  const [gstin, setGstin] = React.useState('');
  const [pan, setPan] = React.useState('');
  const [phone, setPhone] = React.useState('');
  const [email, setEmail] = React.useState('');
  const [website, setWebsite] = React.useState('');

  const [addressLine1, setAddressLine1] = React.useState('');
  const [addressLine2, setAddressLine2] = React.useState('');
  const [city, setCity] = React.useState('');
  const [state, setState] = React.useState('');
  const [postalCode, setPostalCode] = React.useState('');
  const [country, setCountry] = React.useState('India');

  const [beneficiaryName, setBeneficiaryName] = React.useState('');
  const [bankName, setBankName] = React.useState('');
  const [accountNumber, setAccountNumber] = React.useState('');
  const [ifscCode, setIfscCode] = React.useState('');
  const [swiftCode, setSwiftCode] = React.useState('');
  const [bankBranch, setBankBranch] = React.useState('');
  const [bankAddress, setBankAddress] = React.useState('');

  React.useEffect(() => {
    if (currentCompany) {
      setName(currentCompany.name || '');
      setShortCode(currentCompany.shortCode || '');
      setBaseCurrency(currentCompany.baseCurrency || 'INR');
      setIecCode(currentCompany.iecCode || '');
      setGstin(currentCompany.gstin || '');
      setPan(currentCompany.pan || '');
      setPhone(currentCompany.phone || '');
      setEmail(currentCompany.email || '');
      setWebsite(currentCompany.website || '');
      setAddressLine1(currentCompany.addressLine1 || '');
      setAddressLine2(currentCompany.addressLine2 || '');
      setCity(currentCompany.city || '');
      setState(currentCompany.state || '');
      setPostalCode(currentCompany.postalCode || '');
      setCountry(currentCompany.country || 'India');
      setBeneficiaryName(currentCompany.beneficiaryName || '');
      setBankName(currentCompany.bankName || '');
      setAccountNumber(currentCompany.accountNumber || '');
      setIfscCode(currentCompany.ifscCode || '');
      setSwiftCode(currentCompany.swiftCode || '');
      setBankBranch(currentCompany.bankBranch || '');
      setBankAddress(currentCompany.bankAddress || '');
    }
  }, [currentCompany, open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !currentCompany) return;

    setSubmitting(true);
    try {
      const payload: UpdateCompanyInput = {
        id: currentCompany.id,
        name: name.trim(),
        shortCode: shortCode.trim().toUpperCase(),
        baseCurrency: baseCurrency.trim() || 'INR',
        financialYearFrom: currentCompany.financialYearFrom || '04-01',
        financialYearTo: currentCompany.financialYearTo || '03-31',
        iecCode: iecCode.trim(),
        gstin: gstin.trim().toUpperCase(),
        pan: pan.trim().toUpperCase(),
        phone: phone.trim(),
        email: email.trim(),
        website: website.trim(),
        addressLine1: addressLine1.trim(),
        addressLine2: addressLine2.trim(),
        city: city.trim(),
        state: state.trim(),
        postalCode: postalCode.trim(),
        country: country.trim() || 'India',
        beneficiaryName: beneficiaryName.trim() || name.trim(),
        bankName: bankName.trim(),
        accountNumber: accountNumber.trim(),
        ifscCode: ifscCode.trim().toUpperCase(),
        swiftCode: swiftCode.trim().toUpperCase(),
        bankBranch: bankBranch.trim(),
        bankAddress: bankAddress.trim(),
      };

      await updateCompany(payload);
      onOpenChange(false);
    } catch {
      // toast in context
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Building2 className="h-5 w-5" />
            </div>
            <div>
              <DialogTitle className="text-base font-semibold">
                Company Profile &amp; Identity
              </DialogTitle>
              <DialogDescription className="text-xs text-muted-foreground">
                Configures statutory details, customs registrations, and bank instructions for{' '}
                <span className="font-semibold text-foreground">{currentCompany?.name}</span>.
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <Tabs defaultValue="general" className="w-full">
            <TabsList className="grid w-full grid-cols-4">
              <TabsTrigger value="general" className="gap-1.5 text-xs">
                <Building2 className="h-3.5 w-3.5" />
                <span>General</span>
              </TabsTrigger>
              <TabsTrigger value="tax" className="gap-1.5 text-xs">
                <ShieldCheck className="h-3.5 w-3.5" />
                <span>Tax &amp; IEC</span>
              </TabsTrigger>
              <TabsTrigger value="address" className="gap-1.5 text-xs">
                <MapPin className="h-3.5 w-3.5" />
                <span>Address</span>
              </TabsTrigger>
              <TabsTrigger value="banking" className="gap-1.5 text-xs">
                <Landmark className="h-3.5 w-3.5" />
                <span>Banking</span>
              </TabsTrigger>
            </TabsList>

            {/* TAB 1: GENERAL */}
            <TabsContent value="general" className="space-y-3 pt-3">
              <div className="space-y-1.5">
                <Label htmlFor="edit-company-name" className="text-xs font-medium">
                  Company Legal Name <span className="text-destructive">*</span>
                </Label>
                <Input
                  id="edit-company-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="edit-short-code" className="text-xs font-medium">
                    Short Code / Identifier <span className="text-destructive">*</span>
                  </Label>
                  <Input
                    id="edit-short-code"
                    value={shortCode}
                    onChange={(e) => setShortCode(e.target.value.toUpperCase())}
                    maxLength={12}
                    required
                  />
                  <p className="text-[11px] text-muted-foreground">Short abbreviation displayed on top bar badge</p>
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="edit-base-currency" className="text-xs font-medium">
                    Base Currency
                  </Label>
                  <Input
                    id="edit-base-currency"
                    value={baseCurrency}
                    onChange={(e) => setBaseCurrency(e.target.value.toUpperCase())}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="edit-email" className="text-xs font-medium">
                    Contact Email
                  </Label>
                  <Input
                    id="edit-email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="edit-phone" className="text-xs font-medium">
                    Contact Phone
                  </Label>
                  <Input
                    id="edit-phone"
                    value={phone}
                    onChange={(e) => setPhone(e.target.value)}
                  />
                </div>
              </div>
            </TabsContent>

            {/* TAB 2: TAX & REGISTRATIONS */}
            <TabsContent value="tax" className="space-y-3 pt-3">
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="edit-iec-code" className="text-xs font-medium">
                    IEC (Import Export Code)
                  </Label>
                  <Input
                    id="edit-iec-code"
                    placeholder="10-digit DGFT IEC"
                    value={iecCode}
                    onChange={(e) => setIecCode(e.target.value)}
                    maxLength={10}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="edit-gstin" className="text-xs font-medium">
                    GSTIN
                  </Label>
                  <Input
                    id="edit-gstin"
                    placeholder="15-character GSTIN"
                    value={gstin}
                    onChange={(e) => setGstin(e.target.value.toUpperCase())}
                    maxLength={15}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="edit-pan" className="text-xs font-medium">
                    Permanent Account Number (PAN)
                  </Label>
                  <Input
                    id="edit-pan"
                    placeholder="10-digit PAN"
                    value={pan}
                    onChange={(e) => setPan(e.target.value.toUpperCase())}
                    maxLength={10}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="edit-website" className="text-xs font-medium">
                    Website URL
                  </Label>
                  <Input
                    id="edit-website"
                    value={website}
                    onChange={(e) => setWebsite(e.target.value)}
                  />
                </div>
              </div>
            </TabsContent>

            {/* TAB 3: REGISTERED ADDRESS */}
            <TabsContent value="address" className="space-y-3 pt-3">
              <div className="space-y-1.5">
                <Label htmlFor="edit-address-1" className="text-xs font-medium">
                  Address Line 1
                </Label>
                <Textarea
                  id="edit-address-1"
                  value={addressLine1}
                  onChange={(e) => setAddressLine1(e.target.value)}
                  rows={2}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="edit-address-2" className="text-xs font-medium">
                  Address Line 2 (Optional)
                </Label>
                <Input
                  id="edit-address-2"
                  value={addressLine2}
                  onChange={(e) => setAddressLine2(e.target.value)}
                />
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="edit-city" className="text-xs font-medium">
                    City
                  </Label>
                  <Input
                    id="edit-city"
                    value={city}
                    onChange={(e) => setCity(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="edit-state" className="text-xs font-medium">
                    State
                  </Label>
                  <Input
                    id="edit-state"
                    value={state}
                    onChange={(e) => setState(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="edit-postal-code" className="text-xs font-medium">
                    Postal / PIN Code
                  </Label>
                  <Input
                    id="edit-postal-code"
                    value={postalCode}
                    onChange={(e) => setPostalCode(e.target.value)}
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="edit-country" className="text-xs font-medium">
                  Country
                </Label>
                <Input
                  id="edit-country"
                  value={country}
                  onChange={(e) => setCountry(e.target.value)}
                />
              </div>
            </TabsContent>

            {/* TAB 4: BANKING & SWIFT */}
            <TabsContent value="banking" className="space-y-3 pt-3">
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="edit-beneficiary" className="text-xs font-medium">
                    Beneficiary Name
                  </Label>
                  <Input
                    id="edit-beneficiary"
                    value={beneficiaryName}
                    onChange={(e) => setBeneficiaryName(e.target.value)}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="edit-bank-name" className="text-xs font-medium">
                    Bank Name
                  </Label>
                  <Input
                    id="edit-bank-name"
                    value={bankName}
                    onChange={(e) => setBankName(e.target.value)}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="edit-account-num" className="text-xs font-medium">
                    Bank Account Number
                  </Label>
                  <Input
                    id="edit-account-num"
                    value={accountNumber}
                    onChange={(e) => setAccountNumber(e.target.value)}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="edit-ifsc-code" className="text-xs font-medium">
                    IFSC Code
                  </Label>
                  <Input
                    id="edit-ifsc-code"
                    value={ifscCode}
                    onChange={(e) => setIfscCode(e.target.value.toUpperCase())}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="edit-swift-code" className="text-xs font-medium">
                    SWIFT / BIC Code
                  </Label>
                  <Input
                    id="edit-swift-code"
                    value={swiftCode}
                    onChange={(e) => setSwiftCode(e.target.value.toUpperCase())}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="edit-bank-branch" className="text-xs font-medium">
                    Branch Name
                  </Label>
                  <Input
                    id="edit-bank-branch"
                    value={bankBranch}
                    onChange={(e) => setBankBranch(e.target.value)}
                  />
                </div>
              </div>
            </TabsContent>
          </Tabs>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button type="submit" size="sm" disabled={submitting || !name.trim()} className="gap-1.5">
              {submitting ? (
                <>
                  <div className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-primary-foreground border-t-transparent" />
                  <span>Saving...</span>
                </>
              ) : (
                <>
                  <Save className="h-3.5 w-3.5" />
                  <span>Save Profile</span>
                </>
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
