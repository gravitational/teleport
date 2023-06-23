/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { MenuCategoryProps } from "./Category";

const menu: MenuCategoryProps[] = [
  {
    title: "Platform",
    href: "/teleport/",
    testId: "platform",
    containsSubCategories: true,
    children: [
      {
        title: "Platform",
        children: [
          { title: "Why Teleport", href: "/why-teleport/" },
          { title: "How It Works", href: "/how-it-works/" },
        ],
      },
      {
        title: "Access Modules",
        titleLink: true,
        href: "/access-modules/",
        children: [
          { title: "SSH", href: "/access-modules/#ssh" },
          { title: "Kubernetes", href: "/access-modules/#kubernetes" },
          { title: "Databases", href: "/access-modules/#databases" },
          {
            title: "Internal Applications",
            href: "/access-modules/#applications",
          },
          { title: "Windows", href: "/access-modules/#windows" },
        ],
      },
      {
        title: "Our Features",
        href: "/features/",
        titleLink: true,
        children: [
          { title: "Single Sign On", href: "/features/sso-for-ssh/" },
          {
            title: "Just In Time Access Requests",
            href: "/features/access-requests/",
          },
          {
            title: "Role Based Access Control",
            href: "/features/rbac-for-ssh/",
          },
          {
            title: "Audit and Session Recordings",
            href: "/how-it-works/audit-logging-for-ssh-and-kubernetes/",
          },
          {
            title: "Device Trust",
            href: "/docs/access-controls/guides/device-trust/",
            passthrough: false,
          },
          { title: "Passwordless", href: "/passwordless/" },
        ],
      },
    ],
  },
  {
    title: "Solutions",
    href: "/",
    containsSubCategories: true,
    testId: "solutions",
    children: [
      {
        title: "By Industry",
        children: [
          {
            title: "E-commerce & Entertainment",
            href: "/use-cases/ecommerce-entertainment/",
          },
          {
            title: "Financial Services",
            href: "/use-cases/finance/",
          },
          {
            title: "Software-as-a-service (SaaS) Providers",
            href: "/use-cases/saas/",
          },
        ],
      },
      {
        title: "By Use Case",
        children: [
          {
            title: "Privileged Access Management",
            href: "/use-cases/privileged-access-management/",
          },
          {
            title: "Machine-to-Machine Access",
            href: "/use-cases/machine-to-machine-access/",
          },
          {
            title: "Developer-friendly browser",
            href: "/connect/",
          },
          {
            title: "Passwordless Infrastructure Access",
            href: "/passwordless/",
          },
        ],
      },
      {
        title: "By Cloud Provider",
        children: [
          {
            title: "Infrastructure Access for AWS",
            href: "/use-cases/aws/",
          },
        ],
      },
    ],
  },
  { title: "Integrations", href: "/integrations/", testId: "integrations" },
  {
    title: "Documentation",
    href: "/",
    testId: "docs",
    children: [
      {
        title: "Documentation",
        href: "/docs/",
      },
      {
        title: "Download Teleport",
        href: "/download/",
      },
      {
        title: "Teleport Clients",
        href: "/docs/connect-your-client/introduction/",
        passthrough: false,
      },
      {
        title: "How It Works",
        href: "/how-it-works/",
      },
      {
        title: "Teleport Labs",
        href: "/labs/",
      },
      {
        title: "Teleport Community",
        href: "/community/",
      },
      {
        title: "Teleport Slack Channel",
        href: "/slack/",
      },
      {
        title: "GitHub",
        href: "https://github.com/gravitational/teleport/",
      },
    ],
  },
  {
    title: "Learn",
    href: "/resources/",
    testId: "learn",
    children: [
      {
        title: "The Blog",
        href: "/blog/",
      },
      {
        title: "Our Customers",
        href: "/case-study/",
      },
      {
        title: "Resources",
        href: "/resources/",
      },
      {
        title: "Learn",
        href: "/learn/",
      },
      {
        title: "Events",
        href: "/about/events/",
      },
    ],
  },
  {
    title: "Pricing",
    href: "/pricing/",
    testId: "pricing",
  },
  {
    title: "Company",
    href: "/about/",
    testId: "company",
    children: [
      {
        title: "About Us",
        href: "/about/",
      },
      {
        title: "Careers",
        href: "/careers/",
      },
      {
        title: "News",
        href: "/about/press/",
      },
      {
        title: "Partners",
        href: "/partners/",
      },
    ],
  },
];

export default menu;
