import type {Metadata} from 'next';
import Link from 'next/link';

export const metadata: Metadata = {title:'Refund & Cancellation Policy | StoreLift',robots:{index:false,follow:false}};
const contact=process.env.NEXT_PUBLIC_SUPPORT_EMAIL;

export default function Refund(){return <main className="shell max-w-3xl py-16 md:py-24">
  <p className="text-xs font-bold tracking-[.2em] text-blue-600 uppercase">Legal • Draft pending approval</p>
  <h1 className="section-title mt-3">Refund & cancellation policy</h1>
  <p className="mt-6 rounded-xl bg-amber-50 p-4 text-amber-800 text-sm">Draft only. Do not activate live checkout until the merchant approves specific cancellation windows, refund eligibility, governing law and support contact details.</p>
  <section className="mt-9 space-y-6 text-slate-600 leading-relaxed">
    <div><h2 className="text-xl font-bold text-slate-900">Before services begin</h2><p>A client can request cancellation by contacting StoreLift. Eligibility and timing must be set in the approved client agreement and applicable consumer law; this draft does not promise or deny a refund.</p></div>
    <div><h2 className="text-xl font-bold text-slate-900">After work begins</h2><p>The agreed scope, work completed, deliverables received and applicable law determine any refund outcome. Paid service packages may involve manual technical work. Exact rules and any partial-refund calculation must be approved and disclosed before payment.</p></div>
    <div><h2 className="text-xl font-bold text-slate-900">Requesting a refund</h2><p>Contact support with the order reference, store URL and reason for your request. Do not send credit-card numbers or passwords. Refunds, if approved, are processed through the original payment provider and may take additional time to appear.</p></div>
    <div><h2 className="text-xl font-bold text-slate-900">Support</h2><p>{contact ? <a href={`mailto:${contact}`} className="text-blue-600 underline">{contact}</a> : 'Business support email: not configured. This is a launch blocker.'}</p></div>
    <p>For further details, see the <Link href="/terms" className="text-blue-600 underline">Terms of Service</Link>.</p>
  </section>
</main>}
