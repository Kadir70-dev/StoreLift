import type {Metadata} from 'next';
import Link from 'next/link';

export const metadata: Metadata = {title:'Terms of Service | StoreLift',robots:{index:false,follow:false}};
const contact=process.env.NEXT_PUBLIC_SUPPORT_EMAIL;

export default function Terms(){return <main className="shell max-w-3xl py-16 md:py-24">
  <p className="text-xs font-bold tracking-[.2em] text-blue-600 uppercase">Legal • Draft pending approval</p>
  <h1 className="section-title mt-3">Terms of Service</h1>
  <p className="mt-6 rounded-xl bg-amber-50 p-4 text-amber-800 text-sm">Not effective yet. The business owner must confirm the legal seller identity, jurisdiction, contact details, effective date and service/refund terms before launch.</p>
  <section className="mt-9 space-y-6 text-slate-600 leading-relaxed">
    <div><h2 className="text-xl font-bold text-slate-900">What StoreLift provides</h2><p>Our free audit examines a limited number of publicly accessible pages and may include Google PageSpeed measurements. A free audit is diagnostic information, not a full assessment or guarantee. Paid services are scoped and agreed separately before work begins.</p></div>
    <div><h2 className="text-xl font-bold text-slate-900">Merchant permission</h2><p>StoreLift does not need store-admin credentials for free audits. Paid implementation requires the merchant’s explicit approval and appropriately scoped Shopify collaborator permissions. We never ask for the Shopify owner password.</p></div>
    <div><h2 className="text-xl font-bold text-slate-900">Prices and payment</h2><p>Advertised introductory packages are USD $49, $199 and $499. Package scope and any applicable taxes, delivery dates or additional fees must be confirmed before purchasing. Hosted checkout is processed by Stripe; payment is only considered complete after provider verification.</p></div>
    <div><h2 className="text-xl font-bold text-slate-900">No performance guarantee</h2><p>SEO, speed scores, traffic, conversion rate, Google rankings and sales depend on factors outside our control. Recommendations and implementation do not guarantee a particular result.</p></div>
    <div><h2 className="text-xl font-bold text-slate-900">Approvals, refunds and privacy</h2><p>Deliverables, revision limits and acceptance criteria are defined in the client scope. Please review the <Link className="text-blue-600 underline" href="/refund">refund policy</Link> and <Link className="text-blue-600 underline" href="/privacy">privacy notice</Link>; neither is approved for commercial launch yet.</p></div>
    <div><h2 className="text-xl font-bold text-slate-900">Contact</h2><p>{contact ? <a href={`mailto:${contact}`} className="text-blue-600 underline">{contact}</a> : 'Business support email: not configured. This is a launch blocker.'}</p></div>
  </section>
</main>}
