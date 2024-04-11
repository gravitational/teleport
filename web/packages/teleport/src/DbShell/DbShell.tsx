/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, {useState} from 'react';

import {Box, ButtonPrimary, Card, Flex, Input, Text} from "design";
import useTeleport from "teleport/useTeleport";
import {DatabaseQueryRequest} from "teleport/services/databases";

export function DbShell() {
    const ctx = useTeleport();

    const [query, setQuery] = useState('SELECT * FROM pg_catalog.pg_tables');

    const [result, setResult ] = useState(<Text>...</Text>)

    // ctx.databaseService.createDatabase()

    // ctx.databaseService.fetchDatabase()

    async function onSubmitQuery(
        e: React.MouseEvent<HTMLButtonElement>,
    ) {
        e.preventDefault();
        const req : DatabaseQueryRequest = {
            db_service: "postgres",
            db_user: "postgres",
            db_name: "postgres",

            query: query,
        }
        const result = await ctx.databaseService.dbShellQuery("boson.tener.io", req)

        console.log(result)

        if (result.error !== "") {
            setResult(<Text>{result.error}</Text>)
        } else {
            const wholeTable =  (
                    <table style={{ borderCollapse: 'collapse', width: '100%' }}>
                        <thead>
                        {result.headers.map((header, i) => (
                            <th style={{ border: '1px solid #dddddd', backgroundColor: '#f2f2f2', padding: '8px', textAlign: 'left' }} key={i}>{header}</th>
                        ))}
                        </thead>
                        <tbody>
                        {result.query_result.map((row, i) => (
                            <tr key={i}>
                                {row.map((item, index) => (
                                    <td style={{ border: '1px solid #dddddd', padding: '8px', textAlign: 'left' }} key={index}>{item}</td>
                                ))}
                            </tr>
                        ))}
                        </tbody>
                    </table>
            );
            setResult(wholeTable)
        }
    }

  return (
    <Card as="form" mx="auto" width="100%">
                    <Text typography="h3" pt={5} textAlign="center" color="text.main">
                        DB Shell
                    </Text>
                    <Box p={5}>
                        <Input
                            label="Query"
                            value={query}
                            onChange={e => setQuery(e.target.value)}
                        />
                        <ButtonPrimary
                            mt={3}
                            size="large"
                            width="100%"
                            type="submit"
                            onClick={e => onSubmitQuery(e)}
                        >
                            Query
                        </ButtonPrimary>
                        {result}
                    </Box>
    </Card>
  );
}
