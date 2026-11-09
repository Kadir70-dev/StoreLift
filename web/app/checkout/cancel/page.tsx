import Link from 'next/link';
export const metadata={title:'Checkout canceled | StoreLift'};
export default function Cancel(){return <main className="shell py-28 max-w-2xl text-center"><h1 className="section-title">Checkout canceled.</h1><p className="muted mt-6">Your audit request remains saved. No payment has been confirmed by StoreLift. You may revisit the audit or contact the team to continue.</p><Link href="/audit" className="btn btn-primary mt-9">Back to audit</Link></main>}
